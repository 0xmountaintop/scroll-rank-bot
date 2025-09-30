package models

import "time"

// SupplySnapshot holds cached supply and volume data for a coin
type SupplySnapshot struct {
	Circulating    float64   // Circulating supply
	Full           float64   // Full/max supply
	TotalVolumeUSD float64   // 24h volume in USD
	UpdatedAt      time.Time // When this snapshot was created
}

// TTL constants for cache expiration
const (
	SupplyTTL = 24 * time.Hour   // Circulating/Full supply cache lifetime
	VolumeTTL = 30 * time.Minute // Volume cache lifetime
)

// ValidSupply checks if the supply data (Circulating/Full) is still valid
func (s *SupplySnapshot) ValidSupply(now time.Time) bool {
	return now.Sub(s.UpdatedAt) < SupplyTTL
}

// ValidVolume checks if the volume data is still valid
func (s *SupplySnapshot) ValidVolume(now time.Time) bool {
	return now.Sub(s.UpdatedAt) < VolumeTTL
}
