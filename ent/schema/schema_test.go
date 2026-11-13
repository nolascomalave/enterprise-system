package schema

import (
	"strings"
	"testing"

	"enterprise-system/ent/migrate"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type schemaCase struct {
	name        string
	schema      ent.Interface
	table       string
	scope       string
	softDelete  bool
	generatedID bool
}

func schemaCases() []schemaCase {
	return []schemaCase{
		{name: "Subscription", schema: Subscription{}, table: "subscriptions", scope: "global", softDelete: true, generatedID: true},
		{name: "RoleType", schema: RoleType{}, table: "role_types", scope: "global", softDelete: true, generatedID: true},
		{name: "Party", schema: Party{}, table: "parties", scope: "tenants", softDelete: true, generatedID: true},
		{name: "PersonProfile", schema: PersonProfile{}, table: "person_profiles", scope: "tenants", softDelete: true, generatedID: true},
		{name: "OrganizationProfile", schema: OrganizationProfile{}, table: "organization_profiles", scope: "tenants", softDelete: true, generatedID: true},
		{name: "PartyRole", schema: PartyRole{}, table: "party_roles", scope: "tenants", softDelete: true, generatedID: true},
		{name: "PartyMergeLog", schema: PartyMergeLog{}, table: "party_merge_log", scope: "tenants", softDelete: false, generatedID: true},
		{name: "JobFamily", schema: JobFamily{}, table: "job_families", scope: "tenants", softDelete: true, generatedID: true},
		{name: "Job", schema: Job{}, table: "jobs", scope: "tenants", softDelete: true, generatedID: true},
		{name: "Department", schema: Department{}, table: "departments", scope: "tenants", softDelete: true, generatedID: true},
		{name: "Employee", schema: Employee{}, table: "employees", scope: "tenants", softDelete: true, generatedID: false},
		{name: "EmployeeAssignment", schema: EmployeeAssignment{}, table: "employee_assignments", scope: "tenants", softDelete: false, generatedID: true},
		{name: "ContactMechanism", schema: ContactMechanism{}, table: "contact_mechanisms", scope: "global", softDelete: false, generatedID: true},
		{name: "PartyContactMechanism", schema: PartyContactMechanism{}, table: "party_contact_mechanisms", scope: "tenants", softDelete: false, generatedID: true},
		{name: "Address", schema: Address{}, table: "addresses", scope: "tenants", softDelete: true, generatedID: true},
		{name: "Session", schema: Session{}, table: "sessions", scope: "tenants", softDelete: true, generatedID: true},
		{name: "UserCredentials", schema: UserCredentials{}, table: "user_credentials", scope: "tenants", softDelete: true, generatedID: true},
		{name: "PasswordResetToken", schema: PasswordResetToken{}, table: "password_reset_tokens", scope: "tenants", softDelete: false, generatedID: true},
		{name: "SystemFunction", schema: SystemFunction{}, table: "system_functions", scope: "global", softDelete: true, generatedID: true},
		{name: "MenuTemplate", schema: MenuTemplate{}, table: "menu_templates", scope: "global", softDelete: true, generatedID: true},
		{name: "MenuTemplateFunction", schema: MenuTemplateFunction{}, table: "menu_template_functions", scope: "global", softDelete: false, generatedID: true},
		{name: "MenuCategory", schema: MenuCategory{}, table: "menu_categories", scope: "tenants", softDelete: true, generatedID: true},
		{name: "MenuOption", schema: MenuOption{}, table: "menu_options", scope: "tenants", softDelete: true, generatedID: true},
		{name: "MenuOptionFunction", schema: MenuOptionFunction{}, table: "menu_option_functions", scope: "tenants", softDelete: false, generatedID: true},
		{name: "MenuOptionCategory", schema: MenuOptionCategory{}, table: "menu_option_categories", scope: "tenants", softDelete: false, generatedID: true},
		{name: "TenantRole", schema: TenantRole{}, table: "tenant_roles", scope: "tenants", softDelete: true, generatedID: true},
		{name: "RoleMenuOption", schema: RoleMenuOption{}, table: "role_menu_options", scope: "tenants", softDelete: false, generatedID: true},
		{name: "UserRoleAssignment", schema: UserRoleAssignment{}, table: "user_role_assignments", scope: "tenants", softDelete: false, generatedID: true},
		{name: "DepartmentMenuOption", schema: DepartmentMenuOption{}, table: "department_menu_options", scope: "tenants", softDelete: false, generatedID: true},
		{name: "DepartmentMenuOptionValidity", schema: DepartmentMenuOptionValidity{}, table: "department_menu_option_validities", scope: "tenants", softDelete: false, generatedID: true},
		{name: "PositionMenuOption", schema: PositionMenuOption{}, table: "position_menu_options", scope: "tenants", softDelete: false, generatedID: true},
		{name: "PositionMenuOptionValidity", schema: PositionMenuOptionValidity{}, table: "position_menu_option_validities", scope: "tenants", softDelete: false, generatedID: true},
		{name: "EmployeeMenuOptionOverride", schema: EmployeeMenuOptionOverride{}, table: "employee_menu_option_overrides", scope: "tenants", softDelete: false, generatedID: true},
		{name: "TenantRelationship", schema: TenantRelationship{}, table: "tenant_relationships", scope: "global", softDelete: false, generatedID: true},
		{name: "TenantRelationshipInvite", schema: TenantRelationshipInvite{}, table: "tenant_relationship_invites", scope: "global", softDelete: false, generatedID: true},
		{name: "TenantTransfer", schema: TenantTransfer{}, table: "tenant_transfers", scope: "global", softDelete: false, generatedID: true},
	}
}

func TestTableAndSchemaAnnotations(t *testing.T) {
	for _, tc := range schemaCases() {
		t.Run(tc.name, func(t *testing.T) {
			annotations := schemaAnnotations(t, tc.schema)
			assert.Equal(t, tc.table, annotations.Table)
			assert.Equal(t, tc.scope, annotations.Schema)
		})
	}
}

func TestAllSchemasUseUUIDv7PrimaryKeys(t *testing.T) {
	for _, tc := range schemaCases() {
		if !tc.generatedID {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			fields := tc.schema.Fields()
			require.NotEmpty(t, fields)

			id := fields[0].Descriptor()
			require.Equal(t, "id", id.Name)
			require.NotNil(t, id.Default, "id debe tener default UUIDv7")
			require.True(t, id.Immutable, "id debe ser inmutable")

			newID, ok := id.Default.(func() uuid.UUID)
			require.True(t, ok, "default de id debe ser func() uuid.UUID")

			generated := newID()
			assert.NotEqual(t, uuid.Nil, generated)
			assert.Equal(t, uuid.Version(7), generated.Version())
		})
	}
}

func TestTenantTablesAlwaysHaveTenantID(t *testing.T) {
	for _, tc := range schemaCases() {
		if tc.scope != "tenants" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			var found bool
			for _, f := range tc.schema.Fields() {
				if f.Descriptor().Name == "tenant_id" {
					found = true
					break
				}
			}
			assert.True(t, found, "%s debe tener tenant_id", tc.name)
		})
	}
}

func TestPartyTypeEnumValues(t *testing.T) {
	fields := Party{}.Fields()
	partyType := fields[2].Descriptor()

	assert.Equal(t, "party_type", partyType.Name)
	require.Len(t, partyType.Enums, 2)
	assert.Equal(t, "INDIVIDUAL", partyType.Enums[0].V)
	assert.Equal(t, "ORGANIZATION", partyType.Enums[1].V)
}

func TestTenantTablesUseRestrictForeignKeys(t *testing.T) {
	for _, tc := range schemaCases() {
		if tc.scope != "tenants" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			for _, e := range tc.schema.Edges() {
				ant := e.Descriptor().Annotations
				var onDelete string
				for _, a := range ant {
					if sqlAnt, ok := a.(*entsql.Annotation); ok {
						onDelete = string(sqlAnt.OnDelete)
					}
				}
				if onDelete != "" {
					if tc.name == "Party" && e.Descriptor().Name == "sessions" && onDelete == "CASCADE" {
						continue // sessions.user_id is an explicitly justified CASCADE
					}
					assert.Equal(t, "RESTRICT", onDelete, "FK %s debe ser RESTRICT", e.Descriptor().Name)
				}
			}
		})
	}
}

func schemaAnnotations(t *testing.T, s ent.Interface) entsql.Annotation {
	t.Helper()

	for _, a := range s.Annotations() {
		if ant, ok := a.(entsql.Annotation); ok {
			return ant
		}
	}
	t.Fatalf("%T no declara entsql.Annotation", s)
	return entsql.Annotation{}
}

func TestUUIDv7AndTimestampDatabaseDefaults(t *testing.T) {
	for _, tc := range schemaCases() {
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range tc.schema.Fields() {
				d := f.Descriptor()
				switch d.Name {
				case "id":
					if tc.generatedID {
						assertDefaultExpr(t, d, "uuidv7()")
					} else {
						assert.Nil(t, d.Default)
					}
				case "created_at", "updated_at", "merged_at":
					assertDefaultExpr(t, d, "now()")
				}
			}
		})
	}
}

func TestEverySchemaHasSoftDeletePartialIndex(t *testing.T) {
	for _, tc := range schemaCases() {
		if !tc.softDelete {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			var found bool
			for _, idx := range tc.schema.Indexes() {
				for _, a := range idx.Descriptor().Annotations {
					if ant, ok := a.(*entsql.IndexAnnotation); ok && strings.Contains(ant.Where, "deleted_at IS NULL") {
						found = true
					}
				}
			}
			assert.True(t, found, "%s debe tener al menos un índice parcial con deleted_at IS NULL", tc.name)
		})
	}
}

func assertDefaultExpr(t *testing.T, d *field.Descriptor, want string) {
	t.Helper()

	for _, a := range d.Annotations {
		if ant, ok := a.(*entsql.Annotation); ok && ant.DefaultExpr == want {
			return
		}
	}
	t.Errorf("campo %s debe tener DefaultExpr(%q)", d.Name, want)
}

func TestGeneratedForeignKeysAreRestrict(t *testing.T) {
	var checked int
	for _, table := range migrate.Tables {
		for _, fk := range table.ForeignKeys {
			checked++
			if strings.Contains(fk.Symbol, "sessions_parties") {
				assert.Equal(t, schema.Cascade, fk.OnDelete, "sessions.user_id debe ser CASCADE")
				continue
			}
			assert.Equal(t, schema.Restrict, fk.OnDelete, "FK %s debe ser RESTRICT", fk.Symbol)
		}
	}
	assert.NotZero(t, checked, "debe haber FKs generadas")
}

func TestPartyGoldenRecordSchema(t *testing.T) {
	var canonical, status *field.Descriptor
	for _, f := range (Party{}).Fields() {
		switch f.Descriptor().Name {
		case "canonical_party_id":
			canonical = f.Descriptor()
		case "record_status":
			status = f.Descriptor()
		}
	}

	require.NotNil(t, canonical)
	assert.True(t, canonical.Optional)
	assert.True(t, canonical.Nillable)

	require.NotNil(t, status)
	require.Len(t, status.Enums, 3)
	assert.Equal(t, "ACTIVE", status.Enums[0].V)
	assert.Equal(t, "MERGED", status.Enums[1].V)
	assert.Equal(t, "ARCHIVED", status.Enums[2].V)

	edges := map[string]bool{}
	for _, e := range (Party{}).Edges() {
		edges[e.Descriptor().Name] = true
	}
	assert.True(t, edges["canonical_party"])
	assert.True(t, edges["merged_parties"])
}

func TestEmployeePrimaryKeyIsPartyID(t *testing.T) {
	fields := Employee{}.Fields()
	require.NotEmpty(t, fields)

	id := fields[0].Descriptor()
	assert.Equal(t, "id", id.Name)
	assert.Equal(t, "party_id", id.StorageKey)
	assert.Nil(t, id.Default)
	assert.True(t, id.Immutable)
}

func TestSearchableFieldsUseICUCollation(t *testing.T) {
	cases := []struct {
		name   string
		schema ent.Interface
		field  string
	}{
		{name: "job family name", schema: JobFamily{}, field: "name"},
		{name: "job title", schema: Job{}, field: "title"},
		{name: "department name", schema: Department{}, field: "name"},
		{name: "contact value", schema: ContactMechanism{}, field: "value"},
		{name: "location name", schema: Location{}, field: "default_name"},
		{name: "location translation name", schema: LocationTranslation{}, field: "name"},
		{name: "system function name", schema: SystemFunction{}, field: "name"},
		{name: "menu template name", schema: MenuTemplate{}, field: "name"},
		{name: "menu category name", schema: MenuCategory{}, field: "name"},
		{name: "menu option name", schema: MenuOption{}, field: "name"},
		{name: "tenant role name", schema: TenantRole{}, field: "name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var found bool
			for _, f := range tc.schema.Fields() {
				d := f.Descriptor()
				if d.Name != tc.field {
					continue
				}
				for _, a := range d.Annotations {
					if ant, ok := a.(entsql.Annotation); ok && ant.Collation == "unaccent_case_insensitive" {
						found = true
					}
				}
			}
			assert.True(t, found, "%s.%s debe usar unaccent_case_insensitive", tc.name, tc.field)
		})
	}
}

func TestMoneyFieldsUseNumeric(t *testing.T) {
	cases := []struct {
		name   string
		schema ent.Interface
		fields []string
	}{
		{name: "job salaries", schema: Job{}, fields: []string{"min_salary", "max_salary"}},
		{name: "assignment salary", schema: EmployeeAssignment{}, fields: []string{"salary"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range tc.fields {
				var descriptor *field.Descriptor
				for _, f := range tc.schema.Fields() {
					if f.Descriptor().Name == name {
						descriptor = f.Descriptor()
					}
				}
				require.NotNil(t, descriptor, "campo %s", name)
				assert.Equal(t, "numeric(14,2)", descriptor.SchemaType[dialect.Postgres])
			}
		})
	}
}

func TestEmployeeAssignmentHasOpenAssignmentUniqueIndex(t *testing.T) {
	var found bool
	for _, idx := range (EmployeeAssignment{}).Indexes() {
		if !idx.Descriptor().Unique {
			continue
		}
		for _, a := range idx.Descriptor().Annotations {
			if ant, ok := a.(*entsql.IndexAnnotation); ok && strings.Contains(ant.Where, "end_date IS NULL") {
				found = true
			}
		}
	}
	assert.True(t, found, "employee_assignments debe tener unicidad parcial de end_date IS NULL")
}

func TestAddressValidityPartials(t *testing.T) {
	var primary, open bool
	for _, idx := range (AddressValidity{}).Indexes() {
		if !idx.Descriptor().Unique {
			continue
		}
		for _, a := range idx.Descriptor().Annotations {
			ant, ok := a.(*entsql.IndexAnnotation)
			if !ok {
				continue
			}
			if strings.Contains(ant.Where, "is_primary AND end_date IS NULL AND deleted_at IS NULL") {
				primary = true
			}
			if strings.Contains(ant.Where, "end_date IS NULL AND deleted_at IS NULL") &&
				!strings.Contains(ant.Where, "is_primary") {
				open = true
			}
		}
	}
	assert.True(t, primary, "address_validities debe tener una primaria vigente por party/tipo")
	assert.True(t, open, "address_validities debe tener una vigencia abierta por address")
}

func TestPartyContactMechanismValidityPartials(t *testing.T) {
	var primary, open bool
	for _, idx := range (PartyContactMechanismValidity{}).Indexes() {
		if !idx.Descriptor().Unique {
			continue
		}
		for _, a := range idx.Descriptor().Annotations {
			ant, ok := a.(*entsql.IndexAnnotation)
			if !ok {
				continue
			}
			if strings.Contains(ant.Where, "is_primary AND end_date IS NULL") {
				primary = true
			}
			if strings.Contains(ant.Where, "end_date IS NULL") &&
				!strings.Contains(ant.Where, "is_primary") {
				open = true
			}
		}
	}
	assert.True(t, primary, "debe existir un único contacto primario vigente por party/tipo")
	assert.True(t, open, "debe existir una única vigencia abierta por contacto")
}

func TestDepartmentValidityPartials(t *testing.T) {
	assertValidityPartials := func(t *testing.T, indexes []ent.Index, uniquePrimary, uniqueOpen string) {
		var primary, open bool
		for _, idx := range indexes {
			if !idx.Descriptor().Unique {
				continue
			}
			for _, a := range idx.Descriptor().Annotations {
				ant, ok := a.(*entsql.IndexAnnotation)
				if !ok {
					continue
				}
				if ant.Where == uniquePrimary {
					primary = true
				}
				if ant.Where == uniqueOpen {
					open = true
				}
			}
		}
		assert.True(t, primary, "debe existir índice único de primario vigente")
		assert.True(t, open, "debe existir índice único de vigencia abierta")
	}

	t.Run("contacts", func(t *testing.T) {
		assertValidityPartials(t, (DepartmentContactMechanismValidity{}).Indexes(),
			"is_primary AND end_date IS NULL", "end_date IS NULL")
	})
	t.Run("addresses", func(t *testing.T) {
		assertValidityPartials(t, (DepartmentAddressValidity{}).Indexes(),
			"is_primary AND end_date IS NULL", "end_date IS NULL")
	})
}
