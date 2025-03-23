package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/samber/lo"

	cache "github.com/Code-Hex/go-generics-cache"
	gocloak "github.com/Nerzal/gocloak/v13"
)

const (
	defaultCacheExpiration = 30 * time.Second
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

	cacheToken      *cache.Cache[string, string]
	cacheGroup      *cache.Cache[string, map[string]*gocloak.Group]
	cacheGroupUsers *cache.Cache[string, []string]
	keycloakClient  *gocloak.GoCloak
	ctx             context.Context
}

func NewKeycloakClient() KeycloakClientInterface {
	ctx := context.Background()
	clientId := os.Getenv("KEYCLOAK_CLIENT_ID")
	clientSecret := os.Getenv("KEYCLOAK_CLIENT_SECRET")
	realm := os.Getenv("KEYCLOAK_REALM")
	url := os.Getenv("KEYCLOAK_URL")

	cacheToken := cache.New[string, string]()
	cacheGroup := cache.New[string, map[string]*gocloak.Group]()
	cacheGroupUsers := cache.New[string, []string]()
	keycloakClient := gocloak.NewClient(url)

	return &KeycloakClient{
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Realm:        realm,
		Url:          url,

		cacheToken:      cacheToken,
		cacheGroup:      cacheGroup,
		cacheGroupUsers: cacheGroupUsers,
		keycloakClient:  keycloakClient,
		ctx:             ctx,
	}
}

func (k *KeycloakClient) GetToken() (string, error) {
	token, exist := k.cacheToken.Get("token")
	if !exist {
		token, err := k.keycloakClient.LoginClient(k.ctx, k.ClientId, k.ClientSecret, k.Realm)
		if err != nil {
			k.cacheToken.Delete("token")
			return "", err
		}
		k.cacheToken.Set("token", token.AccessToken, cache.WithExpiration(5*time.Minute))
		return token.AccessToken, nil
	}
	return token, nil
}

func (k *KeycloakClient) GetGroupMembers(groupName []string) ([]string, error) {
	token, err := k.GetToken()
	if err != nil {
		return nil, err
	}

	groups, exist := k.cacheGroup.Get("groups")
	if !exist {
		groupsKeycloak, err := k.keycloakClient.GetGroups(k.ctx, token, k.Realm, gocloak.GetGroupsParams{})
		fmt.Println(groupsKeycloak)
		if err != nil {
			k.cacheGroup.Delete("groups")
			return nil, err
		}

		groups = make(map[string]*gocloak.Group)
		lo.ForEach(groupsKeycloak, func(item *gocloak.Group, index int) {
			groups[*item.Name] = item
		})

		k.cacheGroup.Set("groups", groups, cache.WithExpiration(defaultCacheExpiration))
	}

	groupMembers := []string{}
	for _, g := range groupName {
		group := groups[g]
		if group == nil {
			return nil, fmt.Errorf("group %s not exists", g)
		}

		members, exists := k.cacheGroupUsers.Get(k.getGroupKey(*group.Name))
		if !exists {
			membersKeycloak, err := k.keycloakClient.GetGroupMembers(k.ctx, token, k.Realm, *group.ID, gocloak.GetGroupsParams{})
			if err != nil {
				k.cacheGroupUsers.Delete(k.getGroupKey(*group.Name))
				return nil, err
			}
			members = lo.Map(membersKeycloak, func(item *gocloak.User, _ int) string {
				if item.Email == nil {
					return "None"
				}
				return *item.Email
			})
			k.cacheGroupUsers.Set(k.getGroupKey(*group.Name), members, cache.WithExpiration(defaultCacheExpiration))
		}
		groupMembers = append(groupMembers, members...)
	}
	return groupMembers, nil
}

func (k *KeycloakClient) getGroupKey(groupName string) string {
	return fmt.Sprintf("group-%s", groupName)
}
