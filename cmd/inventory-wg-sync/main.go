package main

import (
	"fmt"
	"log"
	"os"

	"github.com/adrg/xdg"

	"github.com/etkecc/inventory-wg-sync/internal/models"
	"github.com/etkecc/inventory-wg-sync/internal/services"
	"github.com/etkecc/inventory-wg-sync/internal/utils"
)

var logger = log.New(os.Stdout, "[inventory-wg-sync] ", 0)

func main() {
	if err := run(); err != nil {
		logger.Fatal(err)
	}
}

func run() error {
	utils.SetLogger(logger)
	path, err := xdg.SearchConfigFile("inventory-wg-sync.yml")
	if err != nil {
		return fmt.Errorf("cannot find the inventory-wg-sync.yml config file: %w, ensure it is in $XDG_CONFIG_DIRS or $XDG_CONFIG_HOME of the root(!) user", err)
	}
	if !utils.IsRoot() {
		logger.Println("WARNING: not running as root, profile updates will fail")
	}

	cfg, err := models.Read(path)
	if err != nil {
		return fmt.Errorf("cannot read the %s config file: %w", path, err)
	}

	utils.SetDebug(cfg.Debug)

	if err := services.Sync(cfg); err != nil {
		return fmt.Errorf("cannot update WireGuard profile: %w", err)
	}
	return nil
}
