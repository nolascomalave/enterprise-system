#!/usr/bin/env python3
"""Generate the enterprise-system database ER diagram as a standalone SVG.

No external dependencies. Run from the repository root:
    python3 docs/database/generate_diagram.py
"""
from html import escape
from pathlib import Path

W, H = 2300, 2380
OUT = Path(__file__).with_name("database-er.svg")

COLORS = {
    "global": {"panel": "#eef2ff", "border": "#c7d2fe", "head": "#3730a3", "line": "#6366f1"},
    "core": {"panel": "#eef2ff", "border": "#c7d2fe", "head": "#4338ca", "line": "#4f46e5"},
    "auth": {"panel": "#fff7ed", "border": "#fed7aa", "head": "#c2410c", "line": "#ea580c"},
    "org": {"panel": "#ecfdf5", "border": "#a7f3d0", "head": "#047857", "line": "#059669"},
    "contacts": {"panel": "#ecfeff", "border": "#a5f3fc", "head": "#0e7490", "line": "#0891b2"},
    "audit": {"panel": "#f8fafc", "border": "#cbd5e1", "head": "#475569", "line": "#64748b"},
}

def esc(v):
    return escape(str(v), quote=True)

def badge_color(kind):
    return {
        "PK": "#f59e0b", "FK": "#3b82f6", "UQ": "#8b5cf6", "PK/FK": "#ef4444",
    }.get(kind, "#94a3b8")

def card_height(cols):
    return 42 + len(cols) * 22 + 32

def card(x, y, w, title, subtitle, cols, color, footer):
    c = COLORS[color]
    h = card_height(cols)
    out = []
    out.append(f'<g id="{esc(title).replace(".", "-")}">')
    out.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="#ffffff" stroke="{c["border"]}" stroke-width="1.5"/>')
    out.append(f'<path d="M{x+10} {y} H{x+w-10} Q{x+w} {y} {x+w} {y+10} V{y+42} H{x} V{y+10} Q{x} {y} {x+10} {y} Z" fill="{c["head"]}"/>')
    out.append(f'<text x="{x+14}" y="{y+18}" font-family="Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif" font-size="16" font-weight="700" fill="#ffffff">{esc(title)}</text>')
    out.append(f'<text x="{x+14}" y="{y+34}" font-family="Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif" font-size="10.5" fill="#e0e7ff">{esc(subtitle)}</text>')
    row_y = y + 42
    for name, typ, kind in cols:
        out.append(f'<rect x="{x+12}" y="{row_y+4}" width="{w-24}" height="18" rx="4" fill="{"#f8fafc" if (cols.index((name,typ,kind)) % 2 == 0) else "#ffffff"}"/>')
        tx = x + 16
        if kind:
            bw = 26 if kind == "PK" or kind == "FK" or kind == "UQ" else 40
            out.append(f'<rect x="{tx}" y="{row_y+6}" width="{bw}" height="14" rx="7" fill="{badge_color(kind)}"/>')
            out.append(f'<text x="{tx+bw/2}" y="{row_y+16.5}" text-anchor="middle" font-family="Inter, sans-serif" font-size="9" font-weight="700" fill="#ffffff">{esc(kind)}</text>')
            tx += bw + 6
        out.append(f'<text x="{tx}" y="{row_y+16.5}" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="12.5" fill="#0f172a">{esc(name)}</text>')
        out.append(f'<text x="{x+w-14}" y="{row_y+16.5}" text-anchor="end" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="10.5" fill="#64748b">{esc(typ)}</text>')
        row_y += 22
    out.append(f'<line x1="{x+1}" y1="{row_y+4}" x2="{x+w-1}" y2="{row_y+4}" stroke="{c["border"]}" stroke-width="1"/>')
    out.append(f'<text x="{x+14}" y="{row_y+21}" font-family="Inter, sans-serif" font-size="10.2" fill="#475569">{esc(footer)}</text>')
    out.append('</g>')
    return "".join(out), h

def panel(x, y, w, h, title, subtitle, color):
    c = COLORS[color]
    return (f'<g><rect x="{x}" y="{y}" width="{w}" height="{h}" rx="18" fill="{c["panel"]}" stroke="{c["border"]}" stroke-width="1.6"/>'
            f'<text x="{x+22}" y="{y+30}" font-family="Inter, sans-serif" font-size="14" font-weight="800" letter-spacing="1.2" fill="{c["head"]}">{esc(title)}</text>'
            f'<text x="{x+22}" y="{y+48}" font-family="Inter, sans-serif" font-size="11" fill="#64748b">{esc(subtitle)}</text></g>')

def line(points, color, dashed=False, label=None, lx=None, ly=None, lw=142):
    c = COLORS[color]
    d = " ".join(("M" if i == 0 else "L") + f"{x} {y}" for i, (x, y) in enumerate(points))
    dash = ' stroke-dasharray="7 5"' if dashed else ""
    out = f'<path d="{d}" fill="none" stroke="{c["line"]}" stroke-width="2"{dash} marker-end="url(#arrow-{color})"/>'
    if label:
        x = lx if lx is not None else points[len(points)//2][0]
        y = ly if ly is not None else points[len(points)//2][1] - 6
        out += (f'<rect x="{x-len(label)*3.25-6}" y="{y-12}" width="{len(label)*6.5+12}" height="17" rx="8" fill="#ffffff" stroke="{c["border"]}"/>'
                f'<text x="{x}" y="{y}" text-anchor="middle" font-family="Inter, sans-serif" font-size="10" font-weight="600" fill="{c["head"]}">{esc(label)}</text>')
    return out

# ---------------------------------------------------------------------------
# Table definitions
# ---------------------------------------------------------------------------
subscriptions = [
    ("id", "uuid", "PK"), ("name", "text", ""), ("slug", "text", "UQ"),
    ("status", "text", ""), ("created_at", "timestamptz", ""),
    ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
role_types = [
    ("id", "uuid", "PK"), ("code", "text", "UQ"), ("name", "text", ""),
    ("is_system", "bool", ""), ("created_at", "timestamptz", ""),
    ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
parties = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("party_type", "text", ""),
    ("display_name", "text", ""), ("canonical_party_id", "uuid", "FK"),
    ("record_status", "text", ""), ("created_at", "timestamptz", ""),
    ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
person_profiles = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("party_id", "uuid", "FK"),
    ("first_name", "text", ""), ("middle_name", "text", ""), ("last_name", "text", ""),
    ("second_last_name", "text", ""), ("birth_date", "timestamptz", ""),
    ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
organization_profiles = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("party_id", "uuid", "FK"),
    ("legal_name", "text", ""), ("trade_name", "text", ""), ("tax_id", "text", ""),
    ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
party_roles = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("party_id", "uuid", "FK"),
    ("role_type_id", "uuid", "FK"), ("created_at", "timestamptz", ""),
    ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
party_merge_log = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("source_party_id", "uuid", "FK"),
    ("target_party_id", "uuid", "FK"), ("merged_at", "timestamptz", ""), ("merged_by", "uuid", ""),
]
sessions = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("user_id", "uuid", "FK"),
    ("token_hash", "bytea", "UQ"), ("expires_at", "timestamptz", ""),
    ("revoked_at", "timestamptz", ""), ("last_used_at", "timestamptz", ""),
    ("metadata", "jsonb", ""), ("created_at", "timestamptz", ""),
    ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
user_credentials = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("party_id", "uuid", "FK"),
    ("party_contact_mechanism_id", "uuid", "FK"), ("password_hash", "text", ""), ("totp_secret", "text", ""),
    ("totp_enabled", "bool", ""), ("backup_codes_hash", "jsonb", ""),
    ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
password_reset_tokens = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("party_id", "uuid", "FK"),
    ("token_hash", "bytea", "UQ"), ("expires_at", "timestamptz", ""),
    ("used_at", "timestamptz", ""), ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""),
]
job_families = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("name", "text", ""),
    ("code", "text", "UQ"), ("description", "text", ""), ("created_at", "timestamptz", ""),
    ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
jobs = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("job_family_id", "uuid", "FK"),
    ("title", "text", ""), ("code", "text", "UQ"), ("description", "text", ""),
    ("min_salary", "numeric(14,2)", ""), ("max_salary", "numeric(14,2)", ""),
    ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
departments = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("parent_department_id", "uuid", "FK"),
    ("name", "text", ""), ("code", "text", "UQ"), ("manager_party_id", "uuid", "FK"),
    ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
employees = [
    ("party_id", "uuid", "PK/FK"), ("tenant_id", "uuid", "FK"), ("employee_code", "text", "UQ"),
    ("hire_date", "timestamptz", ""), ("status", "text", ""), ("job_id", "uuid", "FK"),
    ("department_id", "uuid", "FK"), ("manager_party_id", "uuid", "FK"),
    ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
employee_assignments = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("employee_party_id", "uuid", "FK"),
    ("job_id", "uuid", "FK"), ("department_id", "uuid", "FK"), ("start_date", "timestamptz", ""),
    ("end_date", "timestamptz", ""), ("salary", "numeric(14,2)", ""),
    ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""),
]
contact_mechanisms = [
    ("id", "uuid", "PK"), ("type", "text", ""), ("value", "text", "UQ"),
]
party_contact_mechanisms = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("party_id", "uuid", "FK"),
    ("contact_mechanism_id", "uuid", "FK"), ("is_primary", "bool", ""),
    ("is_verified", "bool", ""), ("contact_mechanism_order", "int", ""),
    ("start_date", "timestamptz", ""), ("end_date", "timestamptz", ""),
    ("created_at", "timestamptz", ""), ("updated_at", "timestamptz", ""),
]
addresses = [
    ("id", "uuid", "PK"), ("tenant_id", "uuid", "FK"), ("party_id", "uuid", "FK"),
    ("type", "text", ""), ("line1", "text", ""), ("line2", "text", ""),
    ("city", "text", ""), ("state", "text", ""), ("postal_code", "text", ""),
    ("country", "text", ""), ("is_primary", "bool", ""), ("created_at", "timestamptz", ""),
    ("updated_at", "timestamptz", ""), ("deleted_at", "timestamptz", ""),
]
change_log = [
    ("id", "uuid", "PK"), ("table_name", "text", ""), ("record_id", "text", ""),
    ("operation", "text", ""), ("old_data", "jsonb", ""), ("new_data", "jsonb", ""),
    ("changed_fields", "jsonb", ""), ("changed_by", "uuid", ""), ("tenant_id", "uuid", ""),
    ("session_id", "uuid", ""), ("changed_at", "timestamptz", "PK"),
]

# ---------------------------------------------------------------------------
# SVG scaffold
# ---------------------------------------------------------------------------
parts = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}">']
parts.append('<defs>')
for name, c in COLORS.items():
    parts.append(f'<marker id="arrow-{name}" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="{c["line"]}"/></marker>')
parts.append('<filter id="shadow" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="2" stdDeviation="3" flood-color="#0f172a" flood-opacity="0.08"/></filter>')
parts.append('</defs>')
parts.append(f'<rect width="{W}" height="{H}" fill="#f8fafc"/>')
parts.append('<rect x="0" y="0" width="2300" height="86" fill="#0f172a"/>')
parts.append('<text x="48" y="37" font-family="Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif" font-size="25" font-weight="800" fill="#ffffff">Enterprise System — Database Structure</text>')
parts.append('<text x="48" y="63" font-family="Inter, sans-serif" font-size="12.5" fill="#cbd5e1">Party Model · PostgreSQL 18 · UUIDv7 · multi-tenant RLS · 19 tables · 2026-11-10</text>')
# legend chips
chips = [("PK", "#f59e0b"), ("FK", "#3b82f6"), ("UQ", "#8b5cf6"), ("RLS", "#4f46e5"), ("AUDIT", "#64748b")]
x = 1770
for label, color in chips:
    parts.append(f'<rect x="{x}" y="24" width="74" height="26" rx="13" fill="#1e293b" stroke="#334155"/>')
    parts.append(f'<circle cx="{x+14}" cy="37" r="4" fill="{color}"/><text x="{x+25}" y="41" font-family="Inter, sans-serif" font-size="10.5" font-weight="700" fill="#e2e8f0">{label}</text>')
    x += 84

# Panels
parts.append(panel(40, 116, 420, 590, "GLOBAL", "catálogo compartido · sin RLS", "global"))
parts.append(panel(490, 116, 950, 960, "TENANT CORE", "RLS ENABLE + FORCE · tenant_id obligatorio", "core"))
parts.append(panel(1460, 116, 820, 1000, "AUTH & SECURITY", "credenciales, sesiones y recuperación", "auth"))
parts.append(panel(40, 730, 420, 1620, "ORG & EMPLOYMENT", "estructura organizacional y historial", "org"))
parts.append(panel(1000, 1080, 440, 770, "CONTACT MDM", "maestro global + relación tenant con RLS", "contacts"))
parts.append(panel(1460, 1136, 820, 370, "AUDIT", "append-only · particionado mensual · sin FKs", "audit"))
parts.append(panel(1460, 1540, 820, 710, "NOTAS", "invariantes y convenciones", "audit"))

# Relationship lines (behind cards)
parts.append(line([(460, 240), (520, 240)], "global", label="1:N", lx=490, ly=228))
parts.append(line([(460, 540), (475, 540), (475, 140), (1040, 140), (1040, 250)], "global", dashed=True, label="1:N role_type_id", lx=760, ly=130))
parts.append(line([(760, 438), (760, 470)], "core", label="1:1", lx=778, ly=458))
parts.append(line([(1000, 350), (1020, 350), (1020, 810), (1000, 810)], "core", label="1:1", lx=1035, ly=760))
parts.append(line([(1000, 240), (1040, 240)], "core", label="1:N", lx=1020, ly=228))
parts.append(line([(1000, 400), (1020, 400), (1020, 500), (1040, 500)], "core", label="source/target", lx=1030, ly=488))
parts.append(line([(520, 300), (500, 300), (500, 200), (520, 200)], "core", label="canonical", lx=480, ly=250))
parts.append(line([(1000, 430), (1020, 430), (1020, 800), (1040, 800)], "core", label="1:N party_id", lx=1028, ly=500))
parts.append(line([(1000, 420), (1020, 420), (1020, 1400), (1040, 1400)], "contacts", label="1:N party_id", lx=1032, ly=1388))
parts.append(line([(1240, 1120), (1240, 996)], "contacts", label="1:N contact_mechanism_id", lx=1240, ly=1108))
parts.append(line([(1440, 850), (1465, 850), (1465, 400), (1500, 400)], "auth", label="1:1 credential", lx=1468, ly=760))
parts.append(line([(1000, 440), (1020, 440), (1020, 1040), (1450, 1040)], "auth", label="user_id / party_id", lx=1235, ly=1028))
parts.append(line([(1450, 300), (1450, 1000)], "auth"))
for yy in (300, 650, 1000):
    parts.append(line([(1450, yy), (1500, yy)], "auth"))
parts.append(line([(520, 438), (480, 438), (480, 1750), (460, 1750)], "core", label="party_id", lx=480, ly=1200))
parts.append(line([(270, 1006), (270, 1040)], "org", label="1:N", lx=285, ly=1028))
parts.append(line([(270, 1352), (270, 1690)], "org", label="1:N job_id", lx=286, ly=1560))
parts.append(line([(380, 1658), (380, 1690)], "org", label="dept", lx=395, ly=1676))
parts.append(line([(460, 1800), (480, 1800), (480, 1900), (460, 1900)], "org", label="manager", lx=485, ly=1855))
parts.append(line([(80, 1500), (60, 1500), (60, 1600), (80, 1600)], "org", label="parent", lx=54, ly=1555))
parts.append(line([(270, 2002), (270, 2030)], "org", label="1:N", lx=285, ly=2020))

# Cards
parts.append(card(80, 170, 380, "global.subscriptions", "tenant root · no RLS", subscriptions, "global", "slug UNIQUE parcial · status ACTIVE/SUSPENDED/CANCELLED")[0])
parts.append(card(80, 440, 380, "global.role_types", "catálogo de roles", role_types, "global", "code UNIQUE parcial · TENANT / ADMIN sembrados")[0])
parts.append(card(520, 170, 480, "tenants.parties", "identidad raíz por tenant", parties, "core", "canonical_party_id self-FK · record_status ACTIVE/MERGED/ARCHIVED")[0])
parts.append(card(520, 470, 480, "tenants.person_profiles", "1:1 para INDIVIDUAL", person_profiles, "core", "party_id UNIQUE · perfil obligatorio para INDIVIDUAL")[0])
parts.append(card(520, 810, 480, "tenants.organization_profiles", "1:1 para ORGANIZATION", organization_profiles, "core", "party_id UNIQUE · tax_id UNIQUE por tenant")[0])
parts.append(card(1040, 170, 400, "tenants.party_roles", "rol por party", party_roles, "core", "UNIQUE (tenant_id, party_id, role_type_id) parcial")[0])
parts.append(card(1040, 430, 400, "tenants.party_merge_log", "log append-only de fusiones", party_merge_log, "core", "source != target · merged_by sin FK")[0])
parts.append(card(1500, 170, 380, "tenants.user_credentials", "login · bcrypt · TOTP", user_credentials, "auth", "party_id UNIQUE · party_contact_mechanism_id UNIQUE · secretos sensibles")[0])
parts.append(card(1500, 520, 380, "tenants.sessions", "refresh token solo hash", sessions, "auth", "token_hash UNIQUE global · user_id CASCADE · expiración")[0])
parts.append(card(1500, 870, 380, "tenants.password_reset_tokens", "tokens de un solo uso", password_reset_tokens, "auth", "token_hash UNIQUE · expires_at · used_at · RLS")[0])
parts.append(card(80, 760, 380, "tenants.job_families", "catálogo de familias", job_families, "org", "code UNIQUE por tenant · name con collation ICU")[0])
parts.append(card(80, 1040, 380, "tenants.jobs", "puestos", jobs, "org", "code UNIQUE por tenant · salary numeric(14,2)")[0])
parts.append(card(80, 1390, 380, "tenants.departments", "jerarquía recursiva", departments, "org", "parent_department_id self-FK · manager_party_id")[0])
parts.append(card(80, 1690, 380, "tenants.employees", "PK = party_id", employees, "org", "PK/FK a parties · status ACTIVE/ON_LEAVE/TERMINATED")[0])
parts.append(card(80, 2030, 380, "tenants.employee_assignments", "historial de asignaciones", employee_assignments, "org", "EXCLUDE GIST sin solapamiento · una asignación abierta")[0])
parts.append(card(1040, 680, 400, "tenants.party_contact_mechanisms", "relación party ↔ contacto con vigencia", party_contact_mechanisms, "core", "UNIQUE contacto activo por tenant · start/end · orden")[0])
parts.append(card(1040, 1120, 400, "global.contact_mechanisms", "maestro global por (type, value)", contact_mechanisms, "global", "UNIQUE (type,value) · ICU · sin tenant/RLS")[0])
parts.append(card(1040, 1300, 400, "tenants.addresses", "legal, fiscal, postal, delivery", addresses, "contacts", "un primario por party+tipo · country ISO-2")[0])
parts.append(card(1500, 1160, 380, "audit.change_log", "evento inmutable · particionado", change_log, "audit", "PK (id, changed_at) · old/new data JSONB · sin FKs")[0])

# Notes panel content
notes = [
    ("Todos los IDs", "uuid PRIMARY KEY DEFAULT uuidv7()"),
    ("Todas las fechas", "timestamptz; created_at / updated_at NOT NULL"),
    ("Soft delete", "deleted_at (excepto merge_log, reset_tokens, party_contact_mechanisms y assignments)"),
    ("Tenant scope", "tenant_id → global.subscriptions(id) ON DELETE RESTRICT"),
    ("RLS", "ENABLE + FORCE; política fail-closed con NULLIF(current_setting(...), '')::uuid"),
    ("Auditoría", "audit.change_log append-only, particionado por changed_at; sin FK a user/session"),
    ("Búsquedas", "collation ICU unaccent_case_insensitive en names/cities/emails"),
    ("Golden Record", "parties.canonical_party_id + resolve_golden_party(uuid)"),
    ("Empleo", "employees.party_id es PK/FK; employee_assignments usa EXCLUDE USING GIST"),
    ("Índices", "todo índice secundario empieza por tenant_id; token_hash es la excepción"),
    ("Contactos", "global.contact_mechanisms (type,value) + relación tenant con vigencia/orden"),
]
yy = 1600
for k, v in notes:
    parts.append(f'<text x="1486" y="{yy}" font-family="Inter, sans-serif" font-size="10.5" font-weight="700" fill="#334155">{esc(k)}</text>')
    parts.append(f'<text x="1592" y="{yy}" font-family="Inter, sans-serif" font-size="10.5" fill="#475569">{esc(v)}</text>')
    yy += 30
parts.append('<text x="1486" y="1940" font-family="Inter, sans-serif" font-size="10.5" font-weight="700" fill="#334155">Relaciones</text>')
parts.append('<text x="1592" y="1940" font-family="Inter, sans-serif" font-size="10.5" fill="#475569">sólida = FK; punteada = tenant scope / RLS</text>')
parts.append('<text x="1486" y="1970" font-family="Inter, sans-serif" font-size="10.5" font-weight="700" fill="#334155">Auth</text>')
parts.append('<text x="1592" y="1970" font-family="Inter, sans-serif" font-size="10.5" fill="#475569">refresh/reset se guardan solo como SHA-256; bcrypt para passwords</text>')
parts.append('<text x="1486" y="2000" font-family="Inter, sans-serif" font-size="10.5" font-weight="700" fill="#334155">Onboarding</text>')
parts.append('<text x="1592" y="2000" font-family="Inter, sans-serif" font-size="10.5" fill="#475569">subscription → tenant context → parties/profiles/roles/contacto/relación/credentials</text>')
parts.append('<text x="1486" y="2030" font-family="Inter, sans-serif" font-size="10.5" font-weight="700" fill="#334155">Party Model</text>')
parts.append('<text x="1592" y="2030" font-family="Inter, sans-serif" font-size="10.5" fill="#475569">sin herencia de tablas; perfiles 1:1 por composición</text>')
parts.append('</svg>')

OUT.write_text("".join(parts), encoding="utf-8")
print(f"wrote {OUT} ({OUT.stat().st_size} bytes)")
