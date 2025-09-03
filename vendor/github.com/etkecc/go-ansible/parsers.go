package ansible

import (
	"strconv"
	"strings"

	"github.com/etkecc/go-kit"
	"golang.org/x/exp/slices"
)

type LineType int

const (
	TypeIgnore        LineType = iota // Line to ignore
	TypeVar           LineType = iota // Line contains var (key=value pair)
	TypeHost          LineType = iota // Line contains host (name key1=value1 key2=value2 ...)
	TypeGroup         LineType = iota // Line contains group ([group])
	TypeGroupVars     LineType = iota // Line contains group vars ([group:vars])
	TypeGroupChild    LineType = iota // Line contains group child (group_child)
	TypeGroupChildren LineType = iota // Line contains group children ([group_children])
)

var groupReplacer = strings.NewReplacer("[", "", "]", "", ":children", "", ":vars", "")

func parseTypeIgnore(line string) bool {
	if line == "" {
		return true
	}
	if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") || line == "" {
		return true
	}
	return false
}

func parseType(line string) LineType {
	line = strings.TrimSpace(line)
	if parseTypeIgnore(line) {
		return TypeIgnore
	}

	if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
		if strings.Contains(line, ":children") {
			return TypeGroupChildren
		}
		if strings.Contains(line, ":vars") {
			return TypeGroupVars
		}
		return TypeGroup
	}

	words := strings.Fields(line)
	if len(words) == 1 {
		if strings.Contains(words[0], "=") {
			return TypeVar
		}
		return TypeGroupChild
	}

	if len(words) == 3 && strings.TrimSpace(words[1]) == "=" { // "key=value", "key =value", "key = value"
		return TypeVar
	}

	return TypeHost
}

func parseGroup(line string) string {
	return groupReplacer.Replace(line)
}

func parseVar(line string) (key, value string) {
	parts := strings.Split(line, "=")
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), kit.Unquote(strings.TrimSpace(parts[1]))
}

func parseHost(line string, only []string) *Host {
	parts := strings.Fields(line)
	hostname := parts[0]
	port := 22
	if (strings.Contains(hostname, "[") &&
		strings.Contains(hostname, "]") &&
		strings.Contains(hostname, ":") &&
		(strings.LastIndex(hostname, "]") < strings.LastIndex(hostname, ":"))) ||
		(!strings.Contains(hostname, "]") && strings.Contains(hostname, ":")) {

		splithost := strings.Split(hostname, ":")
		if i, err := strconv.Atoi(splithost[1]); err == nil && i != 0 {
			port = i
		}
		hostname = splithost[0]
	}
	if len(only) > 0 && !slices.Contains(only, hostname) {
		return nil
	}

	params := parts[1:]

	host := parseParams(params)
	host.Name = hostname
	if host.Host == "" {
		return nil
	}
	if host.Port == 0 {
		host.Port = port
	}
	if host.User == "" {
		host.User = "root"
	}
	return host
}

func parseParams(params []string) *Host {
	vars := &Host{}
	for _, p := range params {
		parts := strings.Split(p, "=")
		if len(parts) < 2 {
			continue
		}
		switch strings.TrimSpace(parts[0]) {
		case "ansible_host":
			vars.Host = kit.Unquote(parts[1])
		case "ansible_port", "ansible_ssh_port":
			vars.Port, _ = strconv.Atoi(kit.Unquote(parts[1])) //nolint:errcheck // should not be a big problem
		case "ansible_user":
			vars.User = kit.Unquote(parts[1])
		case "ansible_ssh_pass":
			vars.SSHPass = kit.Unquote(parts[1])
		case "ansible_ssh_private_key_file":
			vars.PrivateKeys = kit.Uniq(append(vars.PrivateKeys, kit.Unquote(parts[1])))
		case "ansible_become_password":
			vars.BecomePass = kit.Unquote(parts[1])
		case "ordered_at":
			vars.OrderedAt = kit.Unquote(parts[1])
		}
	}

	return vars
}

func parseLimit(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	if !strings.Contains(input, ",") {
		return []string{input}
	}
	parts := strings.Split(input, ",")
	limit := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		limit = append(limit, part)
	}

	return limit
}

func parseDefaultsFromAnsibleCfg(cfg *Cfg) *Host {
	base := &Host{}
	if cfg == nil {
		return base
	}
	if _, ok := cfg.Config["defaults"]; !ok {
		return base
	}

	if user := cfg.Config["defaults"]["remote_user"]; user != "" {
		base.User = user
	}
	if privkey := cfg.Config["defaults"]["private_key_file"]; privkey != "" {
		base.PrivateKeys = kit.Uniq(append(base.PrivateKeys, privkey))
	}
	if port := cfg.Config["defaults"]["remote_port"]; port != "" {
		portI, err := strconv.Atoi(port)
		if err == nil {
			base.Port = portI
		}
	}
	return base
}

func parseAllInventoryPaths(static string, cfg *Cfg) []string {
	all := []string{static}
	if cfg == nil {
		return all
	}

	invcfg := cfg.Config["defaults"]["inventory"]
	if invcfg == "" {
		return all
	}
	invpaths := strings.Split(invcfg, ",")
	if len(invpaths) == 0 {
		return all
	}

	return kit.Uniq(append(all, invpaths...))
}
