package ansible

import (
	"bufio"
	"io"
	"os"
	"path"
	"strings"

	"github.com/etkecc/go-kit"
	"golang.org/x/exp/slices"
)

const (
	defaultGroup = "ungrouped"
	todo         = "todo"
)

// Inventory contains all hosts file content
type Inventory struct {
	cacheGroupVars map[string]*Host    // cached calculated group vars
	cacheGroups    map[string][]string // cached calculated group tree
	only           []string            // limit parsing only to the following hosts

	Groups    map[string][]*Host           // host-by-group
	GroupVars map[string]map[string]string // group vars
	GroupTree map[string][]string          // raw group tree
	Hosts     map[string]*Host             // hosts-by-name
	Paths     []string                     // all inventory paths
}

// Host is a parsed host
type Host struct {
	Vars        HostVars // host vars
	Dirs        []string
	Files       map[string]string
	Group       string   // main group
	Groups      []string // all related groups
	Name        string   // host name
	Host        string   // host address
	Port        int      // host port
	User        string   // host user
	SSHPass     string   // host ssh password
	BecomePass  string   // host become password
	PrivateKeys []string // host ssh private keys
	OrderedAt   string
}

func (h *Host) FindFile(name string) (string, bool) {
	if h == nil || len(h.Dirs) == 0 || len(h.Files) == 0 {
		return "", false
	}

	for _, dir := range h.Dirs {
		fullpath := path.Join(dir, name)
		for k, v := range h.Files {
			if v == fullpath {
				return k, true
			}
		}
	}
	return "", false
}

// HasTODOs returns true if host has any todo values
func (h *Host) HasTODOs() bool {
	if h == nil {
		return true
	}
	if strings.EqualFold(h.Host, todo) {
		return true
	}
	if strings.EqualFold(h.User, todo) {
		return true
	}
	if strings.EqualFold(h.SSHPass, todo) {
		return true
	}
	if strings.EqualFold(h.BecomePass, todo) {
		return true
	}
	if slices.Contains(h.PrivateKeys, todo) {
		return true
	}
	if strings.EqualFold(h.Name, todo) {
		return true
	}

	return h.Vars.HasTODOs()
}

func NewHostsFile(f string, defaults *Host, only ...string) (*Inventory, error) {
	fh, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	hosts := &Inventory{only: only}
	hosts.init()
	hosts.parse(fh, defaults)
	return hosts, nil
}

func (i *Inventory) init() {
	if i == nil {
		return
	}
	if i.cacheGroupVars == nil {
		i.cacheGroupVars = map[string]*Host{}
	}
	if i.cacheGroups == nil {
		i.cacheGroups = map[string][]string{}
	}
	if i.only == nil {
		i.only = make([]string, 0)
	}

	if i.Groups == nil {
		i.Groups = map[string][]*Host{}
		i.Groups[defaultGroup] = make([]*Host, 0)
	}

	if i.GroupTree == nil {
		i.GroupTree = map[string][]string{}
		i.GroupTree[defaultGroup] = make([]string, 0)
	}

	if i.GroupVars == nil {
		i.GroupVars = map[string]map[string]string{}
		i.GroupVars[defaultGroup] = map[string]string{}
	}

	if i.Hosts == nil {
		i.Hosts = map[string]*Host{}
	}
	if i.Paths == nil {
		i.Paths = make([]string, 0)
	}
}

// findAllGroups tries to find all groups related to the group. Experimental
func (i *Inventory) findAllGroups(groups []string) []string {
	cachekey := strings.Join(groups, ",")
	cached := i.cacheGroups[cachekey]
	if cached != nil {
		return cached
	}

	all := groups
	for name, children := range i.GroupTree {
		if slices.Contains(groups, name) {
			all = append(all, name)
			all = append(all, children...)
			continue
		}
		for _, child := range children {
			if slices.Contains(groups, child) {
				all = append(all, name)
				break
			}
		}
	}
	all = kit.Uniq(all)
	if strings.Join(all, ",") != cachekey {
		all = i.findAllGroups(all)
	}
	i.cacheGroups[cachekey] = all

	return all
}

func (i *Inventory) initGroup(name string) {
	if _, ok := i.Groups[name]; !ok {
		i.Groups[name] = make([]*Host, 0)
	}
	if _, ok := i.GroupTree[name]; !ok {
		i.GroupTree[name] = make([]string, 0)
	}
	if _, ok := i.GroupVars[name]; !ok {
		i.GroupVars[name] = make(map[string]string)
	}
}

func (i *Inventory) parse(input io.Reader, defaults *Host) {
	activeGroupName := defaultGroup
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		switch parseType(line) { //nolint:exhaustive // intended
		case TypeGroup:
			activeGroupName = parseGroup(line)
			i.initGroup(activeGroupName)
		case TypeGroupVars:
			activeGroupName = parseGroup(line)
			i.initGroup(activeGroupName)
		case TypeGroupChildren:
			activeGroupName = parseGroup(line)
			i.initGroup(activeGroupName)
		case TypeGroupChild:
			group := parseGroup(line)
			i.initGroup(group)
			i.GroupTree[activeGroupName] = append(i.GroupTree[activeGroupName], group)
		case TypeHost:
			host := parseHost(line, i.only)
			if host != nil {
				host.Group = activeGroupName
				host.Groups = []string{activeGroupName}
				i.Hosts[host.Name] = host
			}
		case TypeVar:
			k, v := parseVar(line)
			i.GroupVars[activeGroupName][k] = v
		}
	}
	i.finalize(defaults)
}

// groupParams converts group vars map[key]value into []string{"key=value"}
func (i *Inventory) groupParams(group string) []string {
	vars := i.GroupVars[group]
	if len(vars) == 0 {
		return nil
	}

	params := make([]string, 0, len(vars))
	for k, v := range vars {
		params = append(params, k+"="+v)
	}
	return params
}

// getGroupVars returns merged group vars. Experimental
func (i *Inventory) getGroupVars(groups []string) *Host {
	cachekey := strings.Join(kit.Uniq(groups), ",")
	cached := i.cacheGroupVars[cachekey]
	if cached != nil {
		return cached
	}

	vars := &Host{}
	for _, group := range groups {
		groupVars := parseParams(i.groupParams(group))
		if groupVars == nil {
			continue
		}
		vars = MergeHost(vars, parseParams(i.groupParams(group)))
	}

	i.cacheGroupVars[cachekey] = vars
	return vars
}

func (i *Inventory) finalize(defaults *Host) {
	for _, host := range i.Hosts {
		host.Groups = i.findAllGroups(kit.Uniq(host.Groups))
		host = MergeHost(host, i.getGroupVars(host.Groups))
		host = MergeHost(host, defaults)
		i.Hosts[host.Name] = host

		for _, group := range host.Groups {
			i.Groups[group] = append(i.Groups[group], host)
		}
	}
}

// Match a host by name
func (i *Inventory) Match(m string) *Host {
	return i.Hosts[m]
}

// Merge does append and replace
//
//nolint:gocognit // intended
func (i *Inventory) Merge(h2 *Inventory) {
	if h2 == nil {
		return
	}

	for k, v := range h2.cacheGroupVars {
		i.cacheGroupVars[k] = v
	}

	for k, v := range h2.cacheGroups {
		i.cacheGroups[k] = v
	}

	i.only = kit.Uniq(append(i.only, h2.only...))

	for group, hosts := range h2.Groups {
		if _, ok := i.Groups[group]; !ok {
			i.Groups[group] = make([]*Host, 0)
		}
		i.Groups[group] = append(i.Groups[group], hosts...)
	}

	for group, vars := range h2.GroupVars {
		if _, ok := i.GroupVars[group]; !ok {
			i.GroupVars[group] = make(map[string]string)
		}
		for k, v := range vars {
			i.GroupVars[group][k] = v
		}
	}

	for group, children := range h2.GroupTree {
		if _, ok := i.GroupTree[group]; !ok {
			i.GroupTree[group] = make([]string, 0)
		}
		i.GroupTree[group] = kit.Uniq(append(i.GroupTree[group], children...))
	}

	i.Paths = kit.Uniq(append(i.Paths, h2.Paths...))

	for group, hosts := range h2.Groups {
		if _, ok := i.Groups[group]; !ok {
			i.Groups[group] = make([]*Host, 0)
		}
		i.Groups[group] = append(i.Groups[group], hosts...)
	}

	for name, host := range h2.Hosts {
		i.Hosts[name] = MergeHost(i.Hosts[name], host)
	}
}
