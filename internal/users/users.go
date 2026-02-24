package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/config"
	"github.com/rusq/slack"
	"github.com/rusq/slackdump/v3"
	"github.com/rusq/slackdump/v3/types"
)

// CachedUser stores only the user ID and Slack handle.
type CachedUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// HandleMap maps user IDs to Slack handles.
type HandleMap map[string]string

// cacheDir returns the path to the users cache directory for a workspace.
func cacheDir(workspaceURL string) (string, error) {
	u, err := url.Parse(workspaceURL)
	if err != nil {
		return "", err
	}
	return filepath.Join(config.CacheDir(), "slackdump", u.Hostname()), nil
}

// cachePath returns the full path to users.json for a workspace.
func cachePath(workspaceURL string) (string, error) {
	dir, err := cacheDir(workspaceURL)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "users.json"), nil
}

// LoadOrFetch loads users from cache, or fetches from the API if the cache
// doesn't exist or force is true. Returns the handle map.
func LoadOrFetch(ctx context.Context, sd *slackdump.Session, workspaceURL string, force bool) (HandleMap, error) {
	path, err := cachePath(workspaceURL)
	if err != nil {
		return nil, err
	}

	if !force {
		m, err := loadCache(path)
		if err == nil {
			slog.Info("loaded cached users", "path", path, "count", len(m))
			return m, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading user cache: %w", err)
		}
	}

	slog.Info("fetching users from Slack API")
	slackUsers, err := fetchUsersPaginated(ctx, sd)
	if err != nil {
		return nil, fmt.Errorf("fetching users: %w", err)
	}

	if err := saveCache(path, slackUsers); err != nil {
		return nil, fmt.Errorf("writing user cache: %w", err)
	}
	slog.Info("cached users", "path", path, "count", len(slackUsers))

	return buildMap(slackUsers), nil
}

// fetchUsersPaginated fetches all users page by page, logging progress
// and respecting Slack rate limits.
func fetchUsersPaginated(ctx context.Context, sd *slackdump.Session) ([]slack.User, error) {
	var all []slack.User
	page := 0
	pager := sd.Client().GetUsersPaginated()
	for {
		page++
		var err error
		pager, err = pager.Next(ctx)
		if pager.Done(err) {
			break
		}
		if err = pager.Failure(err); err != nil {
			var rl *slack.RateLimitedError
			if errors.As(err, &rl) {
				slog.Info("rate limited, waiting", "retry_after", rl.RetryAfter)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(rl.RetryAfter):
				}
				page--
				continue
			}
			return nil, err
		}
		all = append(all, pager.Users...)
		slog.Info("fetching users", "page", page, "fetched", len(pager.Users), "total", len(all))
	}
	return all, nil
}

// loadCache reads CachedUser entries from disk and returns a HandleMap.
func loadCache(path string) (HandleMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cached []CachedUser
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, err
	}
	m := make(HandleMap, len(cached))
	for _, u := range cached {
		m[u.ID] = u.Name
	}
	return m, nil
}

// saveCache writes only IDs and handles to disk.
func saveCache(path string, users types.Users) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	cached := make([]CachedUser, 0, len(users))
	for _, u := range users {
		if u.Name == "" {
			continue
		}
		cached = append(cached, CachedUser{ID: u.ID, Name: u.Name})
	}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// buildMap creates a HandleMap from a slice of slack.User.
func buildMap(users types.Users) HandleMap {
	m := make(HandleMap, len(users))
	for _, u := range users {
		if u.Name != "" {
			m[u.ID] = u.Name
		}
	}
	return m
}

// resolve returns the Slack handle for a user ID, or the original ID if unknown.
func (m HandleMap) resolve(id string) string {
	if name, ok := m[id]; ok {
		return name
	}
	return id
}

var mentionRe = regexp.MustCompile(`<@(U[A-Z0-9]+)>`)

// ResolveConversation replaces user IDs with Slack handles throughout the
// conversation, modifying it in place.
func ResolveConversation(conv *types.Conversation, m HandleMap) {
	for i := range conv.Messages {
		resolveMessage(&conv.Messages[i], m)
	}
}

func resolveMessage(msg *types.Message, m HandleMap) {
	resolveMsg(&msg.Msg, m)
	if msg.SubMessage != nil {
		resolveMsg(msg.SubMessage, m)
	}
	if msg.PreviousMessage != nil {
		resolveMsg(msg.PreviousMessage, m)
	}
	if msg.Root != nil {
		resolveMsg(msg.Root, m)
	}
	for i := range msg.ThreadReplies {
		resolveMessage(&msg.ThreadReplies[i], m)
	}
}

func resolveMsg(msg *slack.Msg, m HandleMap) {
	msg.User = m.resolve(msg.User)
	if msg.Edited != nil {
		msg.Edited.User = m.resolve(msg.Edited.User)
	}
	msg.Inviter = resolveIfSet(msg.Inviter, m)
	msg.ParentUserId = resolveIfSet(msg.ParentUserId, m)
	for i, uid := range msg.ReplyUsers {
		msg.ReplyUsers[i] = m.resolve(uid)
	}
	for i := range msg.Reactions {
		for j, uid := range msg.Reactions[i].Users {
			msg.Reactions[i].Users[j] = m.resolve(uid)
		}
	}
	// Replace <@USERID> mentions in text.
	if strings.Contains(msg.Text, "<@U") {
		msg.Text = mentionRe.ReplaceAllStringFunc(msg.Text, func(match string) string {
			id := mentionRe.FindStringSubmatch(match)[1]
			if name, ok := m[id]; ok {
				return "@" + name
			}
			return match
		})
	}
}

func resolveIfSet(id string, m HandleMap) string {
	if id == "" {
		return ""
	}
	return m.resolve(id)
}
