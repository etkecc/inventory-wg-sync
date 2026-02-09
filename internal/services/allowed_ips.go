package services

import (
	"github.com/etkecc/go-ansible"
	"github.com/etkecc/go-kit"

	"github.com/etkecc/inventory-wg-sync/internal/models"
	"github.com/etkecc/inventory-wg-sync/internal/utils"
)

func AllowedIPs(cfg *models.Config) []string {
	allowedIPs, excludedIPs := configIPs(cfg)
	for _, invPath := range cfg.InventoryPaths {
		allowedIPs = append(allowedIPs, inventoryIPs(invPath, excludedIPs)...)
	}
	allowedIPs = kit.Uniq(allowedIPs)
	utils.SortIPs(allowedIPs)
	return allowedIPs
}

func configIPs(cfg *models.Config) (allowedIPs []string, excludedIPs map[string]bool) {
	excludedIPs = collectExcludedIPs(cfg.ExcludedIPs)
	allowedIPs = collectAllowedIPs(cfg.AllowedIPs, excludedIPs)
	return allowedIPs, excludedIPs
}

func collectExcludedIPs(excluded []string) map[string]bool {
	excludedIPs := map[string]bool{}
	for _, ip := range excluded {
		cidrs := utils.DetermineCIDRs(ip)
		if len(cidrs) == 0 {
			utils.Debug("excluded IP", ip, "is not an IP address")
			continue
		}
		for _, cidr := range cidrs {
			excludedIPs[cidr] = true
		}
	}
	return excludedIPs
}

func collectAllowedIPs(allowed []string, excludedIPs map[string]bool) []string {
	result := make([]string, 0, len(allowed))
	for _, ip := range allowed {
		cidrs := utils.DetermineCIDRs(ip)
		if len(cidrs) == 0 {
			utils.Debug("allowed IP", ip, "is not an IP address")
			continue
		}
		for _, cidr := range cidrs {
			if !excludedIPs[cidr] {
				result = append(result, cidr)
			}
		}
	}
	return result
}

func inventoryIPs(path string, excludedIPs map[string]bool) []string {
	inv, err := ansible.NewHostsFile(path, &ansible.Host{})
	if err != nil {
		utils.Log("ERROR: cannot read inventory file", path, ":", err)
		return nil
	}
	if inv == nil || len(inv.Hosts) == 0 {
		utils.Debug("inventory", path, "is empty")
		return nil
	}
	allowed := make([]string, 0, len(inv.Hosts))
	for _, host := range inv.Hosts {
		allowed = append(allowed, hostAllowedIPs(host.Host, excludedIPs)...)
	}
	return allowed
}

func hostAllowedIPs(host string, excludedIPs map[string]bool) []string {
	cidrs := utils.DetermineCIDRs(host)
	if len(cidrs) == 0 {
		utils.Debug("host", host, "is not an IP address")
		return nil
	}
	allowed := make([]string, 0, len(cidrs))
	for _, cidr := range cidrs {
		if !excludedIPs[cidr] {
			allowed = append(allowed, cidr)
		}
	}
	return allowed
}
