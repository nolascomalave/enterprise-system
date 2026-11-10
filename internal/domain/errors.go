package domain

import "errors"

var (
	ErrInvalidSubscription           = errors.New("invalid subscription")
	ErrInvalidSubscriptionTransition = errors.New("invalid subscription status transition")
	ErrInvalidRoleType               = errors.New("invalid role type")
	ErrInvalidParty                  = errors.New("invalid party")
	ErrPartyNotFound                 = errors.New("party not found")
	ErrInvalidPersonProfile          = errors.New("invalid person profile")
	ErrInvalidOrganizationProfile    = errors.New("invalid organization profile")
	ErrInvalidPartyRole              = errors.New("invalid party role")
	ErrInvalidPartyMerge             = errors.New("invalid party merge")
	ErrPartyTenantMismatch           = errors.New("party merge across tenants")
	ErrPartyNotMergeable             = errors.New("party is not mergeable")
	ErrPartyMergeTargetNotActive     = errors.New("party merge target is not active")
	ErrInvalidJobFamily              = errors.New("invalid job family")
	ErrInvalidJob                    = errors.New("invalid job")
	ErrInvalidDepartment             = errors.New("invalid department")
	ErrInvalidEmployee               = errors.New("invalid employee")
	ErrInvalidEmployeeAssignment     = errors.New("invalid employee assignment")
	ErrEmployeeSelfManager           = errors.New("employee cannot be their own manager")
	ErrInvalidContactMechanism       = errors.New("invalid contact mechanism")
	ErrInvalidAddress                = errors.New("invalid address")
	ErrAlreadyDeleted                = errors.New("record already deleted")
)
