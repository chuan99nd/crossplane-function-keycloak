package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/samber/lo"

	gocloak "github.com/Nerzal/gocloak/v13"
	cache "github.com/patrickmn/go-cache"
)

const (
	defaultCacheExpiration      = 30 * time.Second
	defaultCacheCleanupInterval = 60 * time.Second
)

type KeycloakClientInterface interface {
	GetToken() (string, error)
	GetGroupMembers(groupName []string) ([]string, error)
}

type KeycloakClient struct {
	ClientId     string
	ClientSecret string
	Realm        string
	Url          string

	cacheClient    *cache.Cache
	keycloakClient *gocloak.GoCloak
	ctx            context.Context
}

func NewKeycloakClient() KeycloakClientInterface {
	ctx := context.Background()
	clientId := os.Getenv("KEYCLOAK_CLIENT_ID")
	clientSecret := os.Getenv("KEYCLOAK_CLIENT_SECRET")
	realm := os.Getenv("KEYCLOAK_REALM")
	url := os.Getenv("KEYCLOAK_URL")

	cacheClient := cache.New(defaultCacheExpiration, defaultCacheCleanupInterval)
	keycloakClient := gocloak.NewClient(url)

	return &KeycloakClient{
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Realm:        realm,
		Url:          url,

		cacheClient:    cacheClient,
		keycloakClient: keycloakClient,
		ctx:            ctx,
	}
}

func (k *KeycloakClient) GetToken() (string, error) {
	token, exist := k.cacheClient.Get("token")
	if !exist {
		token, err := k.keycloakClient.LoginClient(k.ctx, k.ClientId, k.ClientSecret, k.Realm)
		if err != nil {
			k.cacheClient.Delete("token")
			return "", err
		}
		k.cacheClient.Set("token", token.AccessToken, 10*time.Minute)
		return token.AccessToken, nil
	}
	return token.(string), nil
}

func (k *KeycloakClient) GetGroupMembers(groupName []string) ([]string, error) {
	token, err := k.GetToken()
	if err != nil {
		return nil, err
	}

	groups, exist := k.cacheClient.Get("groups")
	if !exist {
		groupsKeycloak, err := k.keycloakClient.GetGroups(k.ctx, token, k.Realm, gocloak.GetGroupsParams{})
		if err != nil {
			k.cacheClient.Delete("groups")
			return nil, err
		}
		k.cacheClient.Set("groups", groupsKeycloak, cache.DefaultExpiration)
	}

	if groups == nil {
		return nil, fmt.Errorf("groups not found")
	}

	groupMembers := []string{}
	for _, group := range groups.([]*gocloak.Group) {
		if !lo.Contains(groupName, *group.Name) {
			continue
		}
		members, exist := k.cacheClient.Get(k.getGroupKey(*group.Name))
		membersString := members.([]string)
		if !exist {
			membersKeycloak, err := k.keycloakClient.GetGroupMembers(k.ctx, token, k.Realm, *group.ID, gocloak.GetGroupsParams{})
			if err != nil {
				k.cacheClient.Delete(k.getGroupKey(*group.Name))
				return nil, err
			}
			memberEmails := lo.Map(membersKeycloak, func(member *gocloak.User, index int) string {
				if member.Email == nil {
					return "None"
				}
				return *member.Email
			})
			k.cacheClient.Set(k.getGroupKey(*group.Name), memberEmails, cache.DefaultExpiration)
			membersString = memberEmails
		}
		groupMembers = append(groupMembers, membersString...)
	}
	return groupMembers, nil
}

func (k *KeycloakClient) getGroupKey(groupName string) string {
	return fmt.Sprintf("group-%s", groupName)
}
