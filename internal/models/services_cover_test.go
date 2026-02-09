package models_test

import (
	"testing"

	"github.com/etkecc/inventory-wg-sync/internal/models"
	"github.com/etkecc/inventory-wg-sync/internal/services"
)

func TestServicesCoverage_FromModels(t *testing.T) {
	cfg := &models.Config{}
	if err := services.Sync(cfg); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}
