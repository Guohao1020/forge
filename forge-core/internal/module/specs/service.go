package specs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	effectiveSpecsCacheTTL    = 10 * time.Minute
	effectiveSpecsCachePrefix = "specs:effective:"
)

type Service struct {
	repo  *Repository
	redis *redis.Client
}

func NewService(repo *Repository, redis *redis.Client) *Service {
	return &Service{repo: repo, redis: redis}
}

// ==================== Standards ====================

func (s *Service) ListStandards(ctx context.Context, tenantID int64, f StandardFilter) (*PageResult[*Standard], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.ListStandards(ctx, tenantID, f)
}

func (s *Service) GetStandard(ctx context.Context, tenantID, id int64) (*Standard, error) {
	return s.repo.GetStandard(ctx, tenantID, id)
}

func (s *Service) CreateStandard(ctx context.Context, tenantID, userID int64, req CreateStandardReq) (*Standard, error) {
	result, err := s.repo.CreateStandard(ctx, tenantID, userID, req)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

func (s *Service) UpdateStandard(ctx context.Context, tenantID, id int64, req UpdateStandardReq) (*Standard, error) {
	result, err := s.repo.UpdateStandard(ctx, tenantID, id, req)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

func (s *Service) DeleteStandard(ctx context.Context, tenantID, id int64) error {
	if err := s.repo.DeleteStandard(ctx, tenantID, id); err != nil {
		return err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return nil
}

// ==================== Prompt Templates ====================

func (s *Service) ListPromptTemplates(ctx context.Context, tenantID int64, f PromptTemplateFilter) (*PageResult[*PromptTemplate], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.ListPromptTemplates(ctx, tenantID, f)
}

func (s *Service) GetPromptTemplate(ctx context.Context, tenantID, id int64) (*PromptTemplate, error) {
	return s.repo.GetPromptTemplate(ctx, tenantID, id)
}

func (s *Service) CreatePromptTemplate(ctx context.Context, tenantID, userID int64, req CreatePromptTemplateReq) (*PromptTemplate, error) {
	if req.Variables == nil {
		req.Variables = []string{}
	}
	return s.repo.CreatePromptTemplate(ctx, tenantID, userID, req)
}

func (s *Service) UpdatePromptTemplate(ctx context.Context, tenantID, id int64, req UpdatePromptTemplateReq) (*PromptTemplate, error) {
	if req.Variables == nil {
		req.Variables = []string{}
	}
	return s.repo.UpdatePromptTemplate(ctx, tenantID, id, req)
}

func (s *Service) DeletePromptTemplate(ctx context.Context, tenantID, id int64) error {
	return s.repo.DeletePromptTemplate(ctx, tenantID, id)
}

// ==================== Review Rules ====================

func (s *Service) ListReviewRules(ctx context.Context, tenantID int64, f ReviewRuleFilter) (*PageResult[*ReviewRule], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.ListReviewRules(ctx, tenantID, f)
}

func (s *Service) GetReviewRule(ctx context.Context, tenantID, id int64) (*ReviewRule, error) {
	return s.repo.GetReviewRule(ctx, tenantID, id)
}

func (s *Service) CreateReviewRule(ctx context.Context, tenantID, userID int64, req CreateReviewRuleReq) (*ReviewRule, error) {
	result, err := s.repo.CreateReviewRule(ctx, tenantID, userID, req)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

func (s *Service) UpdateReviewRule(ctx context.Context, tenantID, id int64, req UpdateReviewRuleReq) (*ReviewRule, error) {
	result, err := s.repo.UpdateReviewRule(ctx, tenantID, id, req)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

func (s *Service) ToggleReviewRule(ctx context.Context, tenantID, id int64) (*ReviewRule, error) {
	result, err := s.repo.ToggleReviewRule(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

// ==================== Scaffold Templates ====================

func (s *Service) ListScaffoldTemplates(ctx context.Context, tenantID int64, f ScaffoldFilter) (*PageResult[*ScaffoldTemplate], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.ListScaffoldTemplates(ctx, tenantID, f)
}

func (s *Service) GetScaffoldTemplate(ctx context.Context, tenantID, id int64) (*ScaffoldTemplate, error) {
	return s.repo.GetScaffoldTemplate(ctx, tenantID, id)
}

// ==================== Effective Specs (Three-Level Inheritance) ====================

// GetEffectiveSpecs resolves the three-level inheritance chain: COMPANY -> TEAM -> PROJECT.
// Project-level specs override team-level, which override company-level.
// The result is cached in Redis with a 10-minute TTL.
func (s *Service) GetEffectiveSpecs(ctx context.Context, tenantID, projectID int64) (*EffectiveSpecs, error) {
	cacheKey := fmt.Sprintf("%s%d:%d", effectiveSpecsCachePrefix, tenantID, projectID)

	// Try cache first
	cached, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var result EffectiveSpecs
		if json.Unmarshal(cached, &result) == nil {
			return &result, nil
		}
	}

	// Resolve inheritance: company (scope_id=0) -> project (scope_id=projectID)
	companyStandards, err := s.repo.GetStandardsByScope(ctx, tenantID, "COMPANY", 0)
	if err != nil {
		return nil, fmt.Errorf("get company standards: %w", err)
	}
	projectStandards, err := s.repo.GetStandardsByScope(ctx, tenantID, "PROJECT", projectID)
	if err != nil {
		return nil, fmt.Errorf("get project standards: %w", err)
	}

	mergedStandards := mergeStandards(companyStandards, projectStandards)

	// Resolve review rules similarly
	companyRules, err := s.repo.GetReviewRulesByScope(ctx, tenantID, "COMPANY", 0)
	if err != nil {
		return nil, fmt.Errorf("get company rules: %w", err)
	}
	projectRules, err := s.repo.GetReviewRulesByScope(ctx, tenantID, "PROJECT", projectID)
	if err != nil {
		return nil, fmt.Errorf("get project rules: %w", err)
	}

	mergedRules := mergeRules(companyRules, projectRules)

	result := &EffectiveSpecs{
		Standards: mergedStandards,
		Rules:     mergedRules,
	}

	// Cache the result
	if data, err := json.Marshal(result); err == nil {
		if err := s.redis.Set(ctx, cacheKey, data, effectiveSpecsCacheTTL).Err(); err != nil {
			slog.Warn("failed to cache effective specs", "error", err)
		}
	}

	return result, nil
}

// mergeStandards merges company and project standards. Project-level overrides company-level by category.
func mergeStandards(company, project []*Standard) []*Standard {
	byCategory := make(map[string]*Standard)
	for _, std := range company {
		byCategory[std.Category] = std
	}
	for _, std := range project {
		byCategory[std.Category] = std // override
	}
	result := make([]*Standard, 0, len(byCategory))
	for _, std := range byCategory {
		result = append(result, std)
	}
	return result
}

// mergeRules merges company and project review rules. Project-level overrides company-level by category+name.
func mergeRules(company, project []*ReviewRule) []*ReviewRule {
	ruleKey := func(r *ReviewRule) string { return r.Category + ":" + r.Name }
	byKey := make(map[string]*ReviewRule)
	for _, rule := range company {
		byKey[ruleKey(rule)] = rule
	}
	for _, rule := range project {
		byKey[ruleKey(rule)] = rule // override
	}
	result := make([]*ReviewRule, 0, len(byKey))
	for _, rule := range byKey {
		result = append(result, rule)
	}
	return result
}

// invalidateEffectiveCache removes all effective specs cache entries for a tenant.
func (s *Service) invalidateEffectiveCache(ctx context.Context, tenantID int64) {
	pattern := fmt.Sprintf("%s%d:*", effectiveSpecsCachePrefix, tenantID)
	iter := s.redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := s.redis.Del(ctx, iter.Val()).Err(); err != nil {
			slog.Warn("failed to invalidate cache", "key", iter.Val(), "error", err)
		}
	}
}
