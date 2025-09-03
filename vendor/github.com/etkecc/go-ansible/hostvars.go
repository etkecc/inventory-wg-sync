package ansible

import (
	"os"
	"regexp"
	"strings"

	"github.com/etkecc/go-kit"
	"golang.org/x/exp/slices"
	yaml "gopkg.in/yaml.v3"
)

const cachePrefix = "__cache_"

type HostVars map[string]any

var (
	notTemplated    = regexp.MustCompile("{{.*}}")
	allowedBoolVals = map[string]struct{}{
		"yes":   {},
		"no":    {},
		"true":  {},
		"false": {},
		"True":  {},
		"False": {},
	}
)

func NewHostVarsFile(f string) (HostVars, error) {
	fh, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	var vars map[string]any
	if err := yaml.NewDecoder(fh).Decode(&vars); err != nil {
		return nil, err
	}
	precacheDomain(vars)
	return vars, nil
}

func precacheDomain(hv HostVars) {
	base, domain := hv.Domain()
	hv[cachePrefix+"domain"] = domain
	hv[cachePrefix+"base"] = base
}

// HasTODOs returns true if there are any TODOs in hostvars
func (hv HostVars) HasTODOs() bool {
	for k := range hv {
		if strings.EqualFold(k, todo) || strings.EqualFold(hv.string(k), todo) {
			return true
		}
	}
	return false
}

// String returns string value
func (hv HostVars) String(key string, optionalDefault ...string) string {
	value := hv.string(key, optionalDefault...)
	base, domain := hv.Domain()
	return strings.ReplaceAll(strings.ReplaceAll(value, "{{ matrix_domain }}", domain), "{{ base_domain }}", base)
}

// StringSlice returns string slice value
func (hv HostVars) StringSlice(key string) []string {
	var zero []string
	v, ok := hv[key]
	if !ok {
		return zero
	}

	vhv, ok := v.([]any)
	if !ok {
		return zero
	}

	value := []string{}
	for _, v := range vhv {
		vStr, ok := v.(string)
		if ok {
			value = append(value, vStr)
		}
	}

	return value
}

// Bool returns bool value
func (hv HostVars) Bool(key string) *bool {
	v, ok := hv[key]
	if !ok {
		return nil
	}
	value, ok := v.(bool)
	if ok {
		return &value
	}
	str, ok := v.(string)
	if !ok {
		return nil
	}
	if _, ok := allowedBoolVals[str]; !ok {
		return nil
	}

	value = str == "yes" || str == "true" || str == "True"
	return &value
}

// Yes returns true if value is true (bool, string) or missing (if missing = true)
func (hv HostVars) Yes(missing bool, key string) bool {
	vBool := hv.Bool(key)
	if vBool != nil {
		return *vBool
	}
	return missing
}

func (hv HostVars) OSUser() string {
	return hv.string("matrix_user_name", "matrix")
}

func (hv HostVars) OSGroup() string {
	return hv.string("matrix_group_name", "matrix")
}

func (hv HostVars) OSPath() string {
	return hv.string("matrix_base_data_path", "/matrix")
}

// FQN attempts to parse FQN var and replaces {{ matrix_domain }}, {{ base_domain }}, etc.
func (hv HostVars) FQN(key string) string {
	base, domain := hv.Domain()
	v := hv.String("matrix_server_fqn_" + key)
	if v == "" {
		v = hv.String(key + "_hostname")
	}

	if v == "" {
		if key == "grafana" {
			key = "stats"
		}
		if key == "element_call" {
			key = "call"
		}
		if base != "" {
			return key + "." + base
		}
		return key + "." + domain
	}

	if repase := hv.mustReparse(v); repase != "" {
		repase = strings.TrimSuffix(strings.TrimPrefix(repase, "matrix_server_fqn_"), "_hostname")
		return hv.FQN(repase)
	}

	return v
}

// Domain returns base domain (if exists) and domain
func (hv HostVars) Domain() (base, domain string) {
	if cachedDomain := hv.string(cachePrefix + "domain"); cachedDomain != "" {
		return hv.string(cachePrefix + "base"), cachedDomain
	}

	base = strings.TrimSpace(hv.string("base_domain"))
	domain = strings.ReplaceAll(strings.TrimSpace(hv.string("matrix_domain")), "{{ base_domain }}", base)

	return base, domain
}

// Admin parses admin MXID
func (hv HostVars) Admin() string {
	return hv.String("matrix_admin")
}

// Admins parses admin MXIDs
func (hv HostVars) Admins() []string {
	admins := map[string]struct{}{}
	if admin := hv.Admin(); admin != "" {
		admins[admin] = struct{}{}
	}

	base, domain := hv.Domain()
	hAdmins := hv.StringSlice("matrix_admins")
	for _, hAdmin := range hAdmins {
		hAdmin = strings.ReplaceAll(strings.ReplaceAll(hAdmin, "{{ matrix_domain }}", domain), "{{ base_domain }}", base)
		admins[hAdmin] = struct{}{}
	}

	slice := make([]string, 0, len(admins))
	for admin := range admins {
		slice = append(slice, admin)
	}

	return slice
}

// IsAdmin checks if provided input is server admin
func (hv HostVars) IsAdmin(input string) bool {
	return slices.Contains(hv.Admins(), input)
}

// Email returns email
func (hv HostVars) Email() string {
	keys := []string{"etke_service_monitoring_email", "etke_order_email", "etke_subscription_email"}
	for _, key := range keys {
		if email := hv.string(key); email != "" {
			return email
		}
	}
	return ""
}

// Emails returns all emails
func (hv HostVars) Emails() []string {
	emails := []string{}
	keys := []string{"etke_service_monitoring_email", "etke_order_email", "etke_subscription_email"}
	for _, key := range keys {
		if email := hv.string(key); email != "" {
			emails = append(emails, email)
		}
	}
	emails = append(emails, hv.StringSlice("etke_subscription_emails")...)
	return kit.Uniq(emails)
}

// MaintenanceEnabled returns bool
func (hv HostVars) MaintenanceEnabled() bool {
	keys := []string{"etke_service_maintenance_enabled", "injector_recurring_auto"}
	for _, key := range keys {
		if enabled := hv.Bool(key); enabled != nil {
			return *enabled
		}
	}
	return true
}

// MaintenanceBranch returns docker tag
func (hv HostVars) MaintenanceBranch() string {
	keys := []string{"etke_service_maintenance_branch", "injector_playbook_tag"}
	for _, key := range keys {
		if branch := hv.string(key); branch != "" {
			return branch
		}
	}
	return "latest"
}

func (hv HostVars) mustReparse(value string) string {
	if !notTemplated.MatchString(value) {
		return ""
	}

	return strings.TrimSpace(strings.Replace(strings.Replace(value, "{{", "", 1), "}}", "", 1))
}

func (hv HostVars) string(key string, optionalDefault ...string) string {
	var zero string

	if len(optionalDefault) > 0 {
		zero = optionalDefault[0]
	}

	v, ok := hv[key]
	if !ok {
		return zero
	}
	value, ok := v.(string)
	if !ok {
		return zero
	}

	return value
}
