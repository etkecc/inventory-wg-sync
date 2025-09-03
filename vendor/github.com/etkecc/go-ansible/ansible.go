package ansible

import (
	"errors"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/etkecc/go-kit"
	yaml "gopkg.in/yaml.v3"
)

// ParseInventory using ansible.cfg and hosts (ini) files
func ParseInventory(ansibleCfg, hostsini, limit string) *Inventory {
	if ansibleCfg == "" {
		ansibleCfg = path.Join(path.Dir(path.Dir(hostsini)), "/ansible.cfg")
	}

	acfg, err := NewAnsibleCfgFile(ansibleCfg)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println("cannot parse", ansibleCfg, "error:", err)
		return nil
	}

	paths := parseAllInventoryPaths(hostsini, acfg)
	defaults := parseDefaultsFromAnsibleCfg(acfg)
	inv := parseHostsFiles(paths, parseLimit(limit), defaults)
	if inv == nil {
		log.Println("no hosts found in inventories:", paths)
		return nil
	}

	groupVars := getUsedGroupsVars(paths, inv)
	wg := kit.NewWaitGroup()
	for _, host := range inv.Hosts {
		wg.Do(func() {
			inv.Hosts[host.Name].Dirs, inv.Hosts[host.Name].Files = parseAdditionalFiles(paths, host.Name)

			vars := parseHostVars(paths, groupVars[host.Group], host.Name)
			if vars == nil {
				return
			}
			inv.Hosts[host.Name].Vars = vars
			if sshPrivateKey := inv.Hosts[host.Name].Vars.String("ansible_ssh_private_key_file"); sshPrivateKey != "" {
				inv.Hosts[host.Name].PrivateKeys = kit.Uniq(append(inv.Hosts[host.Name].PrivateKeys, sshPrivateKey))
			}
		})
	}
	wg.Wait()
	return inv
}

func parseHostsFiles(paths, only []string, defaults *Host) *Inventory {
	inv := &Inventory{}
	inv.init()

	for _, path := range paths {
		parsedInv, err := NewHostsFile(path, defaults, only...)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", path, "error:", err)
			continue
		}
		if parsedInv != nil {
			parsedInv.Paths = []string{path}
		}

		inv.Merge(parsedInv)
	}

	if len(inv.Hosts) == 0 {
		return nil
	}

	return inv
}

// getUsedGroupsVars returns map of group name to its vars for all actually used groups in inventory
func getUsedGroupsVars(paths []string, inv *Inventory) map[string]HostVars {
	// find all actually used groups
	usedGroups := map[string]bool{}
	for _, host := range inv.Hosts {
		usedGroups[host.Group] = true
	}

	allGroupsVars := map[string]HostVars{}
	for usedGroup := range usedGroups {
		groupTree := inv.cacheGroups[usedGroup]
		if len(groupTree) == 0 {
			// set empty value to avoid dealing with nil later
			allGroupsVars[usedGroup] = HostVars{}
		}
		kit.Reverse(groupTree) // we need specific order to properly override vars
		groupVars := HostVars{}
		for _, group := range groupTree {
			for k, v := range parseGroupVars(paths, group) {
				groupVars[k] = v
			}
		}
		allGroupsVars[usedGroup] = groupVars
	}

	return allGroupsVars
}

// parseGroupVars returns group vars for a group (the first found file wins)
func parseGroupVars(hostsPaths []string, group string) HostVars {
	allpaths := []string{}
	for _, hostsPath := range hostsPaths {
		groupPath := path.Join(path.Dir(hostsPath), "/group_vars/", group)
		// if groupPath file exists, add to list
		if _, err := os.Stat(groupPath); err == nil {
			allpaths = append(allpaths, groupPath)
		}
	}
	allpaths = kit.Uniq(allpaths)

	for _, groupPath := range allpaths {
		vars, err := parseGroupVarsFile(groupPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", groupPath, "error:", err)
			continue
		}
		if vars != nil {
			return vars
		}
	}

	return nil
}

// parseGroupVarsFile is basically the same as NewHostVarsFile,
// but without any optimizations or tricks of caching
func parseGroupVarsFile(groupVarsPath string) (HostVars, error) {
	fh, err := os.Open(groupVarsPath)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	var vars map[string]any
	if err := yaml.NewDecoder(fh).Decode(&vars); err != nil {
		return nil, err
	}
	return vars, nil
}

func parseHostVars(hostsPaths []string, groupVars HostVars, name string) HostVars {
	allvars := []HostVars{}
	for _, hostsPath := range hostsPaths {
		varsPath := path.Join(path.Dir(hostsPath), "/host_vars/", name, "/vars.yml")
		vars, err := NewHostVarsFile(varsPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", varsPath, "error:", err)
			continue
		}
		allvars = append(allvars, vars)
	}
	if len(allvars) == 0 {
		return nil
	}

	final := HostVars{}
	for k, v := range groupVars {
		final[k] = v
	}

	for _, vars := range allvars {
		for k, v := range vars {
			final[k] = v
		}
	}
	return final
}

// parseAdditionalFiles returns list of dirs to create, map of files (source => target) and error
func parseAdditionalFiles(invPaths []string, name string) (dirs []string, files map[string]string) {
	hostvarsDir := path.Join("/host_vars", name)
	groupvarsDir := "/group_vars"

	dirs = []string{hostvarsDir, groupvarsDir}
	files = map[string]string{}

	for _, invPath := range invPaths {
		hostvarsPath := path.Join(path.Dir(invPath), hostvarsDir)
		groupvarsPath := path.Join(path.Dir(invPath), groupvarsDir)
		pDirs, pFiles, err := findFilesAndDirs(hostvarsPath, hostvarsDir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", hostvarsPath, "error:", err)
			continue
		}
		dirs = append(dirs, pDirs...)
		for k, v := range pFiles {
			files[k] = v
		}
		gDirs, gFiles, err := findFilesAndDirs(groupvarsPath, groupvarsDir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", groupvarsPath, "error:", err)
			continue
		}

		dirs = append(dirs, gDirs...)
		for k, v := range gFiles {
			files[k] = v
		}

	}

	return kit.Uniq(dirs), files
}

func findFilesAndDirs(src, prepend string) (dirs []string, files map[string]string, err error) {
	dirs = []string{}
	files = map[string]string{}
	err = filepath.Walk(src, func(itempath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if itempath == src {
			return nil
		}
		name := path.Join(prepend, strings.Replace(itempath, src, "", 1))
		if info.IsDir() {
			dirs = append(dirs, name)
			return nil
		}

		files[itempath] = name
		return nil
	})
	return dirs, files, err
}
