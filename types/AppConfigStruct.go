package types

import (
	"PluginRPC/utils"

	"errors"
	"os"
)

type AppConfig struct {
	AppId           string
	Endpoint        string
	AppPort         uint16
	ManagementToken string
	StorageDir      string

	PlatformOverlayEndpoint           string
	PlatformPlaystationEndpoint       string
	PlatformPlaystationMobileEndpoint string
}

func NewAppConfig() AppConfig {
	return AppConfig{
		AppId:           utils.Or(os.Getenv("APP_ID"), "1483998531797782718"),
		Endpoint:        utils.Or(os.Getenv("DS_ENDPOINT"), "https://discord.com/api/v10"),
		AppPort:         utils.StrTPort(utils.Or(os.Getenv("APP_PORT"), "3000")),
		ManagementToken: utils.Or(os.Getenv("MANAGEMENT_TOKEN"), "SuperSecretPassword"),
		StorageDir:      utils.Or(os.Getenv("APP_STORAGE_DIR"), "store"),

		PlatformOverlayEndpoint:           utils.Or(os.Getenv("PLATFORM_OVERLAY_ENDPOINT"), "https://raw.githubusercontent.com/RegularRabbit05/PluginRPCStatic/refs/heads/main/v1/tmdb/%s"),
		PlatformPlaystationEndpoint:       utils.Or(os.Getenv("PLATFORM_PLAYSTATION_ENDPOINT"), "https://api.serialstation.com/v1/tmdb/%s"),
		PlatformPlaystationMobileEndpoint: utils.Or(os.Getenv("PLATFORM_PLAYSTATION_ENDPOINT"), "https://api.serialstation.com/v1/title-ids/%s"),
	}
}

func (c *AppConfig) IsValid() error {
	if c.AppId == "0" {
		return errors.New("app id is empty")
	}

	if c.Endpoint == "" {
		return errors.New("endpoint is empty")
	}

	if c.AppPort == 0 {
		return errors.New("app port is invalid")
	}

	if c.ManagementToken == "" {
		return errors.New("management token is empty")
	}

	if c.StorageDir == "" {
		return errors.New("storage dir is empty")
	}

	if c.PlatformOverlayEndpoint == "" {
		return errors.New("overlay endpoint is empty")
	}

	if c.PlatformPlaystationEndpoint == "" {
		return errors.New("playstation endpoint is empty")
	}

	if c.PlatformPlaystationMobileEndpoint == "" {
		return errors.New("playstation mobile endpoint is empty")
	}

	return nil
}
