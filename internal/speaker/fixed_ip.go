package speaker

// FixedIPReloader aktualisiert feste Raumname->IP-Overrides zur Laufzeit
// (Sonos-Treiber). Mock und andere Treiber implementieren das nicht.
type FixedIPReloader interface {
	ReloadFixedIPs(overrides map[string]string)
}
