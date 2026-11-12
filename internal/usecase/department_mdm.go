package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type AddDepartmentContactInput struct {
	TenantID     uuid.UUID
	DepartmentID uuid.UUID
	Type         domain.ContactType
	Value        string
	Order        int
	IsPrimary    bool
}

type UpdateDepartmentContactValidityInput struct {
	TenantID            uuid.UUID
	DepartmentID        uuid.UUID
	DepartmentContactID uuid.UUID
	IsPrimary           bool
}

type AddDepartmentAddressInput struct {
	TenantID       uuid.UUID
	DepartmentID   uuid.UUID
	CityLocationID uuid.UUID
	Type           domain.AddressType
	Line1          string
	Line2          string
	IsPrimary      bool
}

type UpdateDepartmentAddressValidityInput struct {
	TenantID            uuid.UUID
	DepartmentID        uuid.UUID
	DepartmentAddressID uuid.UUID
	IsPrimary           bool
}

type DepartmentContactResult struct {
	Contact  *domain.ContactMechanism
	Relation *domain.DepartmentContactMechanism
	Validity *domain.DepartmentContactMechanismValidity
}

type DepartmentAddressResult struct {
	Address  *domain.DepartmentAddress
	Validity *domain.DepartmentAddressValidity
}

type DepartmentMDMService struct {
	repo domain.DepartmentMDMRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewDepartmentMDMService(repo domain.DepartmentMDMRepository, tx TransactionManager, now func() time.Time) *DepartmentMDMService {
	if now == nil {
		now = time.Now
	}
	return &DepartmentMDMService{repo: repo, tx: tx, now: now}
}

func (s *DepartmentMDMService) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s == nil || s.repo == nil || s.tx == nil {
		return errors.New("department mdm: nil dependencies")
	}
	return s.tx.WithinTransaction(ctx, fn)
}

func (s *DepartmentMDMService) AddContact(ctx context.Context, in AddDepartmentContactInput) (*DepartmentContactResult, error) {
	contact, err := domain.NewContactMechanism(in.Type, in.Value)
	if err != nil {
		return nil, validationError("invalid_department_contact", "invalid contact mechanism", err)
	}
	if in.TenantID == uuid.Nil || in.DepartmentID == uuid.Nil || in.Order < 0 {
		return nil, validationError("invalid_department_contact", "invalid department contact", domain.ErrInvalidDepartmentContact)
	}

	var result *DepartmentContactResult
	err = s.withinTx(ctx, func(ctx context.Context) error {
		found, err := s.repo.FindOrCreateContactMechanism(ctx, contact)
		if err != nil {
			return fmt.Errorf("find or create contact mechanism: %w", err)
		}
		now := s.now().UTC()
		relation, err := domain.NewDepartmentContactMechanism(
			in.TenantID, in.DepartmentID, found.ID, found.Type, in.Order, in.IsPrimary, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateDepartmentContact(ctx, relation); err != nil {
			return fmt.Errorf("create department contact: %w", err)
		}
		if in.IsPrimary {
			if err := s.closeOpenPrimaryContact(ctx, in.TenantID, in.DepartmentID, found.Type, relation.ID, now); err != nil {
				return err
			}
		}
		validity, err := domain.NewDepartmentContactMechanismValidity(
			in.TenantID, relation.ID, in.DepartmentID, found.Type, in.IsPrimary, now, nil, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateDepartmentContactValidity(ctx, validity); err != nil {
			return fmt.Errorf("create department contact validity: %w", err)
		}
		result = &DepartmentContactResult{Contact: found, Relation: relation, Validity: validity}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DepartmentMDMService) UpdateContactValidity(ctx context.Context, in UpdateDepartmentContactValidityInput) (*DepartmentContactResult, error) {
	if in.TenantID == uuid.Nil || in.DepartmentID == uuid.Nil || in.DepartmentContactID == uuid.Nil {
		return nil, validationError("invalid_department_contact", "ids are required", domain.ErrInvalidDepartmentContact)
	}
	var result *DepartmentContactResult
	err := s.withinTx(ctx, func(ctx context.Context) error {
		relation, err := s.repo.GetDepartmentContact(ctx, in.TenantID, in.DepartmentContactID)
		if err != nil {
			return fmt.Errorf("get department contact: %w", err)
		}
		if relation.DepartmentID != in.DepartmentID {
			return domain.ErrDepartmentContactNotFound
		}
		now := s.now().UTC()
		if open, err := s.repo.GetOpenDepartmentContactValidity(ctx, in.TenantID, relation.ID); err == nil {
			if err := s.repo.CloseDepartmentContactValidity(ctx, in.TenantID, open.ID, now); err != nil {
				return fmt.Errorf("close current contact validity: %w", err)
			}
		} else if !errors.Is(err, domain.ErrDepartmentContactNotFound) {
			return fmt.Errorf("get current contact validity: %w", err)
		}
		if in.IsPrimary {
			if err := s.closeOpenPrimaryContact(ctx, in.TenantID, in.DepartmentID, relation.ContactType, relation.ID, now); err != nil {
				return err
			}
		}
		validity, err := domain.NewDepartmentContactMechanismValidity(
			in.TenantID, relation.ID, in.DepartmentID, relation.ContactType, in.IsPrimary, now, nil, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateDepartmentContactValidity(ctx, validity); err != nil {
			return fmt.Errorf("create department contact validity: %w", err)
		}
		result = &DepartmentContactResult{Relation: relation, Validity: validity}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DepartmentMDMService) AddAddress(ctx context.Context, in AddDepartmentAddressInput) (*DepartmentAddressResult, error) {
	address, err := domain.NewDepartmentAddress(
		in.TenantID, in.DepartmentID, in.CityLocationID, in.Type, in.Line1, in.Line2, s.now(),
	)
	if err != nil {
		return nil, validationError("invalid_department_address", "invalid department address", err)
	}
	var result *DepartmentAddressResult
	err = s.withinTx(ctx, func(ctx context.Context) error {
		now := s.now().UTC()
		if err := s.repo.CreateDepartmentAddress(ctx, address); err != nil {
			return fmt.Errorf("create department address: %w", err)
		}
		if in.IsPrimary {
			if err := s.closeOpenPrimaryAddress(ctx, in.TenantID, in.DepartmentID, address.Type, address.ID, now); err != nil {
				return err
			}
		}
		validity, err := domain.NewDepartmentAddressValidity(
			in.TenantID, address.ID, in.DepartmentID, address.Type, in.IsPrimary, now, nil, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateDepartmentAddressValidity(ctx, validity); err != nil {
			return fmt.Errorf("create department address validity: %w", err)
		}
		result = &DepartmentAddressResult{Address: address, Validity: validity}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DepartmentMDMService) UpdateAddressValidity(ctx context.Context, in UpdateDepartmentAddressValidityInput) (*DepartmentAddressResult, error) {
	if in.TenantID == uuid.Nil || in.DepartmentID == uuid.Nil || in.DepartmentAddressID == uuid.Nil {
		return nil, validationError("invalid_department_address", "ids are required", domain.ErrInvalidDepartmentAddress)
	}
	var result *DepartmentAddressResult
	err := s.withinTx(ctx, func(ctx context.Context) error {
		address, err := s.repo.GetDepartmentAddress(ctx, in.TenantID, in.DepartmentAddressID)
		if err != nil {
			return fmt.Errorf("get department address: %w", err)
		}
		if address.DepartmentID != in.DepartmentID {
			return domain.ErrDepartmentAddressNotFound
		}
		now := s.now().UTC()
		if open, err := s.repo.GetOpenDepartmentAddressValidity(ctx, in.TenantID, address.ID); err == nil {
			if err := s.repo.CloseDepartmentAddressValidity(ctx, in.TenantID, open.ID, now); err != nil {
				return fmt.Errorf("close current address validity: %w", err)
			}
		} else if !errors.Is(err, domain.ErrDepartmentAddressNotFound) {
			return fmt.Errorf("get current address validity: %w", err)
		}
		if in.IsPrimary {
			if err := s.closeOpenPrimaryAddress(ctx, in.TenantID, in.DepartmentID, address.Type, address.ID, now); err != nil {
				return err
			}
		}
		validity, err := domain.NewDepartmentAddressValidity(
			in.TenantID, address.ID, in.DepartmentID, address.Type, in.IsPrimary, now, nil, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateDepartmentAddressValidity(ctx, validity); err != nil {
			return fmt.Errorf("create department address validity: %w", err)
		}
		result = &DepartmentAddressResult{Address: address, Validity: validity}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DepartmentMDMService) closeOpenPrimaryContact(
	ctx context.Context,
	tenantID, departmentID uuid.UUID,
	contactType domain.ContactType,
	excludeContactID uuid.UUID,
	at time.Time,
) error {
	open, err := s.repo.GetOpenPrimaryDepartmentContactValidity(ctx, tenantID, departmentID, contactType)
	if errors.Is(err, domain.ErrDepartmentContactNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get open primary department contact: %w", err)
	}
	if open.DepartmentContactMechanismID == excludeContactID {
		return nil
	}
	if err := s.repo.CloseDepartmentContactValidity(ctx, tenantID, open.ID, at); err != nil {
		return fmt.Errorf("close open primary department contact: %w", err)
	}
	return nil
}

func (s *DepartmentMDMService) closeOpenPrimaryAddress(
	ctx context.Context,
	tenantID, departmentID uuid.UUID,
	addressType domain.AddressType,
	excludeAddressID uuid.UUID,
	at time.Time,
) error {
	open, err := s.repo.GetOpenPrimaryDepartmentAddressValidity(ctx, tenantID, departmentID, addressType)
	if errors.Is(err, domain.ErrDepartmentAddressNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get open primary department address: %w", err)
	}
	if open.DepartmentAddressID == excludeAddressID {
		return nil
	}
	if err := s.repo.CloseDepartmentAddressValidity(ctx, tenantID, open.ID, at); err != nil {
		return fmt.Errorf("close open primary department address: %w", err)
	}
	return nil
}
