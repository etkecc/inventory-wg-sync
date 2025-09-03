package ansible

import (
	"bufio"
	"io"
	"os"
	"strings"
)

const defaultSection = "unknown"

// Cfg represents the ansible.cfg file
type Cfg struct {
	Config map[string]map[string]string
}

func NewAnsibleCfgFile(f string) (*Cfg, error) {
	if f == "" {
		return nil, nil
	}
	fh, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	cfg := &Cfg{}
	cfg.parse(fh)
	return cfg, nil
}

func (a *Cfg) parse(input io.Reader) {
	a.Config = make(map[string]map[string]string)

	activeSectionName := defaultSection
	scanner := bufio.NewScanner(input)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch parseType(line) { //nolint:exhaustive // intended
		case TypeGroup:
			activeSectionName = parseGroup(line)
		case TypeVar:
			key, value := parseVar(line)
			if _, ok := a.Config[activeSectionName]; !ok {
				a.Config[activeSectionName] = map[string]string{}
			}
			a.Config[activeSectionName][key] = value
		}
	}
}
