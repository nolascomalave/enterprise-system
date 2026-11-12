package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

// PartyNameResult returns the MDM value together with its current validity.
type PartyNameResult struct {
	Name     *domain.PartyName
	Validity *domain.PartyNameValidity
}

type AddPartyNameInput struct {
	TenantID  uuid.UUID
	PartyID   uuid.UUID
	NameType  string
	Value     string
	IsPrimary bool
	Ordering  int
}

type UpdatePartyNameValidityInput struct {
	TenantID    uuid.UUID
	PartyID     uuid.UUID
	PartyNameID uuid.UUID
	NewValue    string
	IsPrimary   bool
	Ordering    int
}

type PartyNameService struct {
	repo domain.PartyNameRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewPartyNameService(repo domain.PartyNameRepository, tx TransactionManager, now func() time.Time) *PartyNameService {
	if now == nil {
		now = time.Now
	}
	return &PartyNameService{repo: repo, tx: tx, now: now}
}

func (s *PartyNameService) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s == nil || s.repo == nil || s.tx == nil {
		return errors.New("party names: nil dependencies")
	}
	return s.tx.WithinTransaction(ctx, fn)
}

// Add creates the name if it does not exist and ensures it has an open
// validity. Adding the same value again is idempotent unless the caller asks
// to promote it to primary.
func (s *PartyNameService) Add(ctx context.Context, in AddPartyNameInput) (*PartyNameResult, error) {
	code := domain.PartyNameTypeCode(strings.ToUpper(strings.TrimSpace(in.NameType)))
	if !code.Valid() {
		return nil, validationError("invalid_party_name", "invalid name_type", domain.ErrInvalidPartyName)
	}
	value := strings.TrimSpace(in.Value)
	if in.TenantID == uuid.Nil || in.PartyID == uuid.Nil || value == "" || in.Ordering < 0 {
		return nil, validationError("invalid_party_name", "invalid party name", domain.ErrInvalidPartyName)
	}

	var result *PartyNameResult
	err := s.withinTx(ctx, func(ctx context.Context) error {
		nameType, err := s.repo.FindNameTypeByCode(ctx, string(code))
		if err != nil {
			return fmt.Errorf("find name type: %w", err)
		}
		now := s.now().UTC()

		name, err := s.repo.FindPartyName(ctx, in.TenantID, in.PartyID, nameType.ID, value)
		switch {
		case errors.Is(err, domain.ErrPartyNameNotFound):
			name, err = domain.NewPartyName(
				in.TenantID, in.PartyID, nameType.ID, value, in.IsPrimary, in.Ordering, now,
			)
			if err != nil {
				return err
			}
			if err := s.repo.CreatePartyName(ctx, name); err != nil {
				return fmt.Errorf("create party name: %w", err)
			}
		case err != nil:
			return fmt.Errorf("find party name: %w", err)
		default:
			if err := name.Update(value, in.IsPrimary, in.Ordering, now); err != nil {
				return err
			}
			if err := s.repo.UpdatePartyName(ctx, name); err != nil {
				return fmt.Errorf("update party name: %w", err)
			}
		}

		open, err := s.repo.GetOpenValidity(ctx, in.TenantID, name.ID)
		switch {
		case err == nil && open.IsPrimary == in.IsPrimary:
			result = &PartyNameResult{Name: name, Validity: open}
			return nil
		case err == nil:
			if err := s.repo.CloseValidity(ctx, in.TenantID, open.ID, now); err != nil {
				return fmt.Errorf("close previous validity: %w", err)
			}
		case !errors.Is(err, domain.ErrPartyNameValidityNotFound):
			return fmt.Errorf("get open validity: %w", err)
		}

		if in.IsPrimary {
			if err := s.closeOpenPrimary(ctx, in.TenantID, in.PartyID, nameType.ID, name.ID, now); err != nil {
				return err
			}
		}

		validity, err := domain.NewPartyNameValidity(
			in.TenantID, name.ID, in.PartyID, nameType.ID, in.IsPrimary, now, nil, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateValidity(ctx, validity); err != nil {
			return fmt.Errorf("create party name validity: %w", err)
		}
		result = &PartyNameResult{Name: name, Validity: validity}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateValidity closes the current validity and opens a new one. The new
// window may keep the same value or switch to a new/previous value.
func (s *PartyNameService) UpdateValidity(ctx context.Context, in UpdatePartyNameValidityInput) (*PartyNameResult, error) {
	if in.TenantID == uuid.Nil || in.PartyID == uuid.Nil || in.PartyNameID == uuid.Nil || in.Ordering < 0 {
		return nil, validationError("invalid_party_name", "invalid party name validity", domain.ErrInvalidPartyName)
	}

	var result *PartyNameResult
	err := s.withinTx(ctx, func(ctx context.Context) error {
		current, err := s.repo.GetPartyName(ctx, in.TenantID, in.PartyNameID)
		if err != nil {
			return fmt.Errorf("get party name: %w", err)
		}
		if current.PartyID != in.PartyID {
			return domain.ErrPartyNameNotFound
		}
		now := s.now().UTC()

		if open, err := s.repo.GetOpenValidity(ctx, in.TenantID, current.ID); err == nil {
			if err := s.repo.CloseValidity(ctx, in.TenantID, open.ID, now); err != nil {
				return fmt.Errorf("close current validity: %w", err)
			}
		} else if !errors.Is(err, domain.ErrPartyNameValidityNotFound) {
			return fmt.Errorf("get current validity: %w", err)
		}

		target, err := s.resolveTargetName(ctx, current, in, now)
		if err != nil {
			return err
		}

		if target.ID != current.ID {
			if open, err := s.repo.GetOpenValidity(ctx, in.TenantID, target.ID); err == nil {
				if err := s.repo.CloseValidity(ctx, in.TenantID, open.ID, now); err != nil {
					return fmt.Errorf("close target validity: %w", err)
				}
			} else if !errors.Is(err, domain.ErrPartyNameValidityNotFound) {
				return fmt.Errorf("get target validity: %w", err)
			}
		}

		if in.IsPrimary {
			if err := s.closeOpenPrimary(ctx, in.TenantID, in.PartyID, target.NameTypeID, target.ID, now); err != nil {
				return err
			}
		}

		validity, err := domain.NewPartyNameValidity(
			in.TenantID, target.ID, in.PartyID, target.NameTypeID, in.IsPrimary, now, nil, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateValidity(ctx, validity); err != nil {
			return fmt.Errorf("create party name validity: %w", err)
		}
		result = &PartyNameResult{Name: target, Validity: validity}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PartyNameService) resolveTargetName(
	ctx context.Context,
	current *domain.PartyName,
	in UpdatePartyNameValidityInput,
	now time.Time,
) (*domain.PartyName, error) {
	value := strings.TrimSpace(in.NewValue)
	if value == "" {
		value = current.Value
	}
	if strings.EqualFold(value, current.Value) {
		if err := current.Update(current.Value, in.IsPrimary, in.Ordering, now); err != nil {
			return nil, err
		}
		if err := s.repo.UpdatePartyName(ctx, current); err != nil {
			return nil, fmt.Errorf("update party name: %w", err)
		}
		return current, nil
	}

	target, err := s.repo.FindPartyName(ctx, in.TenantID, in.PartyID, current.NameTypeID, value)
	switch {
	case errors.Is(err, domain.ErrPartyNameNotFound):
		target, err = domain.NewPartyName(
			in.TenantID, in.PartyID, current.NameTypeID, value, in.IsPrimary, in.Ordering, now,
		)
		if err != nil {
			return nil, err
		}
		if err := s.repo.CreatePartyName(ctx, target); err != nil {
			return nil, fmt.Errorf("create target party name: %w", err)
		}
		return target, nil
	case err != nil:
		return nil, fmt.Errorf("find target party name: %w", err)
	default:
		if err := target.Update(value, in.IsPrimary, in.Ordering, now); err != nil {
			return nil, err
		}
		if err := s.repo.UpdatePartyName(ctx, target); err != nil {
			return nil, fmt.Errorf("update target party name: %w", err)
		}
		return target, nil
	}
}

func (s *PartyNameService) closeOpenPrimary(
	ctx context.Context,
	tenantID, partyID, nameTypeID, excludeNameID uuid.UUID,
	at time.Time,
) error {
	open, err := s.repo.GetOpenPrimaryValidity(ctx, tenantID, partyID, nameTypeID)
	if errors.Is(err, domain.ErrPartyNameValidityNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get open primary validity: %w", err)
	}
	if open.PartyNameID == excludeNameID {
		return nil
	}
	if err := s.repo.CloseValidity(ctx, tenantID, open.ID, at); err != nil {
		return fmt.Errorf("close open primary validity: %w", err)
	}
	return nil
}
