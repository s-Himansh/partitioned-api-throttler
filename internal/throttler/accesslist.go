package throttler

import "sync"

type AccessList struct {
	mu        sync.RWMutex
	whitelist map[string]bool
	blacklist map[string]bool
}

func NewAccessList() *AccessList {
	return &AccessList{
		whitelist: make(map[string]bool),
		blacklist: make(map[string]bool),
	}
}

func (al *AccessList) IsWhitelisted(ip string) bool {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return al.whitelist[ip]
}

func (al *AccessList) IsBlacklisted(ip string) bool {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return al.blacklist[ip]
}

func (al *AccessList) AddWhitelist(ip string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.whitelist[ip] = true
}

func (al *AccessList) RemoveWhitelist(ip string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	delete(al.whitelist, ip)
}

func (al *AccessList) AddBlacklist(ip string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.blacklist[ip] = true
}

func (al *AccessList) RemoveBlacklist(ip string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	delete(al.blacklist, ip)
}

func (al *AccessList) GetWhitelist() []string {
	al.mu.RLock()
	defer al.mu.RUnlock()
	out := make([]string, 0, len(al.whitelist))
	for ip := range al.whitelist {
		out = append(out, ip)
	}
	return out
}

func (al *AccessList) GetBlacklist() []string {
	al.mu.RLock()
	defer al.mu.RUnlock()
	out := make([]string, 0, len(al.blacklist))
	for ip := range al.blacklist {
		out = append(out, ip)
	}
	return out
}
