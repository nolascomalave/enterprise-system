package domain

import (
	"time"

	"github.com/google/uuid"
)

var (
	testTenantID   = uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000001")
	testPartyID    = uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000002")
	testRoleTypeID = uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000003")
)

func testNow() time.Time {
	return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
}
