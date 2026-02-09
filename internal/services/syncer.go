package services

import (
	"github.com/etkecc/inventory-wg-sync/internal/models"
	"github.com/etkecc/inventory-wg-sync/internal/utils"
)

func Sync(cfg *models.Config) error {
	allowedIPs := AllowedIPs(cfg)
	utils.Log("discovered", len(allowedIPs), "allowed IPs")
	if len(allowedIPs) == 0 {
		utils.Log("WARNING: no allowed IPs found")
	}
	if len(allowedIPs) == 0 {
		return nil
	}

	return SyncWireGuard(cfg, allowedIPs)
}
