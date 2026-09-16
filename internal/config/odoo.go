package config

import "errors"

type OdooConfig struct {
	DBURL  string `json:"db_url,omitempty"`
	Token  string `json:"token,omitempty"`
	DbUUID string `json:"db_uuid,omitempty"`
}

func (cm *Manager) GetOdooConfig() OdooConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.Odoo
}

func (cm *Manager) SaveOdooCredentials(dbURL, token, dbUUID string) error {
	if dbURL == "" || token == "" {
		return errors.New("dbURL and token cannot be empty")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.Data.Odoo.DBURL == dbURL && cm.Data.Odoo.Token == token && cm.Data.Odoo.DbUUID == dbUUID {
		return nil
	}

	cm.Data.Odoo = OdooConfig{DBURL: dbURL, Token: token, DbUUID: dbUUID}
	return cm.saveLocked()
}

func (cm *Manager) ClearOdooConfig() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.Data.Odoo = OdooConfig{}
	return cm.saveLocked()
}

func (cm *Manager) HasOdooCredentials() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.Data.Odoo.DBURL != "" && cm.Data.Odoo.Token != ""
}
