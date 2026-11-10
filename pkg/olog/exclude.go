// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Implements regex-based exclusion of log records by
// message and/or attribute value, configured from the olog
// configuration file.

package olog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync/atomic"
)

// ErrInvalidExclude is returned by [ConfigureFromFile] (and surfaced
// through [PollConfigurationFile]'s callback) for each exclude rule that
// has an invalid regular expression or specifies neither a message nor
// attributes. Valid rules are still applied.
var ErrInvalidExclude = errors.New("invalid exclude rule")

// ExcludeConfig is a single exclusion rule. A record is dropped, at any
// level, when it matches every field that is set on the rule. A rule
// must set at least one of Message or Attrs; to silence a logger
// entirely, set its level to OFF instead.
type ExcludeConfig struct {
	// Address optionally scopes the rule to loggers whose package or
	// module path equals it. Empty applies to all loggers.
	Address string `yaml:"address,omitempty"`

	// Message is an optional RE2 regular expression matched against the
	// log message.
	Message string `yaml:"message,omitempty"`

	// Attrs optionally maps attribute keys to RE2 regular expressions
	// matched against the attribute's string value. Attributes nested in
	// groups use dotted keys (e.g. "http.route"). All must match.
	Attrs map[string]string `yaml:"attrs,omitempty"`
}

// compiledRule is a validated, compiled ExcludeConfig.
type compiledRule struct {
	address string
	message *regexp.Regexp
	attrs   map[string]*regexp.Regexp
}

// excludeRules holds the process-wide compiled rules, replaced
// wholesale on each configuration load.
var excludeRules atomic.Pointer[[]compiledRule]

// compileExcludes compiles cfgs, returning the valid rules and an error
// joining one ErrInvalidExclude per rejected rule.
func compileExcludes(cfgs []ExcludeConfig, path string) ([]compiledRule, error) {
	var (
		rules []compiledRule
		errs  []error
	)
	for i := range cfgs {
		r, err := compileExclude(&cfgs[i])
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: rule %d in %s: %w", ErrInvalidExclude, i, path, err))
			continue
		}
		rules = append(rules, r)
	}
	return rules, errors.Join(errs...)
}

func compileExclude(c *ExcludeConfig) (compiledRule, error) {
	if c.Message == "" && len(c.Attrs) == 0 {
		return compiledRule{}, errors.New("must set message or attrs")
	}
	r := compiledRule{address: c.Address}
	if c.Message != "" {
		re, err := regexp.Compile(c.Message)
		if err != nil {
			return compiledRule{}, fmt.Errorf("message: %w", err)
		}
		r.message = re
	}
	if len(c.Attrs) > 0 {
		r.attrs = make(map[string]*regexp.Regexp, len(c.Attrs))
		for k, v := range c.Attrs {
			re, err := regexp.Compile(v)
			if err != nil {
				return compiledRule{}, fmt.Errorf("attrs[%q]: %w", k, err)
			}
			r.attrs[k] = re
		}
	}
	return r, nil
}

// excludeHandler drops records matching the global exclusion rules
// before they reach the wrapped handler.
type excludeHandler struct {
	inner slog.Handler
	addrs []string

	// bound are attributes added via WithAttrs, with keys already
	// prefixed by the group path active when they were added.
	bound []slog.Attr

	// prefix is the dotted group path (with trailing dot) applied to
	// record attributes.
	prefix string
}

// newExcludeHandler wraps inner with exclusion support for loggers
// identified by addrs.
func newExcludeHandler(inner slog.Handler, addrs []string) slog.Handler {
	return &excludeHandler{inner: inner, addrs: addrs}
}

// Enabled implements slog.Handler.
func (h *excludeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle implements slog.Handler.
//
//nolint:gocritic // Why: Handle's signature is fixed by the slog.Handler interface.
func (h *excludeHandler) Handle(ctx context.Context, r slog.Record) error {
	if rp := excludeRules.Load(); rp != nil && len(*rp) > 0 && h.excluded(*rp, &r) {
		return nil
	}
	return h.inner.Handle(ctx, r)
}

func (h *excludeHandler) excluded(rules []compiledRule, r *slog.Record) bool {
	var attrs map[string]string // built lazily
	for i := range rules {
		rule := &rules[i]
		if !h.inScope(rule.address) {
			continue
		}
		if rule.message != nil && !rule.message.MatchString(r.Message) {
			continue
		}
		if len(rule.attrs) > 0 {
			if attrs == nil {
				attrs = h.collectAttrs(r)
			}
			if !attrsMatch(rule.attrs, attrs) {
				continue
			}
		}
		return true
	}
	return false
}

func (h *excludeHandler) inScope(address string) bool {
	if address == "" {
		return true
	}
	for _, a := range h.addrs {
		if a == address {
			return true
		}
	}
	return false
}

func attrsMatch(want map[string]*regexp.Regexp, got map[string]string) bool {
	for k, re := range want {
		v, ok := got[k]
		if !ok || !re.MatchString(v) {
			return false
		}
	}
	return true
}

// collectAttrs flattens bound and record attributes into dotted keys
// with resolved string values. Record attributes win over bound ones.
func (h *excludeHandler) collectAttrs(r *slog.Record) map[string]string {
	out := make(map[string]string, len(h.bound)+r.NumAttrs())
	for _, a := range h.bound {
		flattenAttr("", a, out)
	}
	r.Attrs(func(a slog.Attr) bool {
		flattenAttr(h.prefix, a, out)
		return true
	})
	return out
}

func flattenAttr(prefix string, a slog.Attr, out map[string]string) {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		p := prefix
		if a.Key != "" {
			p += a.Key + "."
		}
		for _, ga := range v.Group() {
			flattenAttr(p, ga, out)
		}
		return
	}
	if a.Key == "" {
		return
	}
	out[prefix+a.Key] = v.String()
}

// WithAttrs implements slog.Handler.
func (h *excludeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	n := *h
	n.inner = h.inner.WithAttrs(attrs)
	n.bound = append(append([]slog.Attr(nil), h.bound...), prefixAttrs(h.prefix, attrs)...)
	return &n
}

// WithGroup implements slog.Handler.
func (h *excludeHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	n := *h
	n.inner = h.inner.WithGroup(name)
	n.prefix = h.prefix + name + "."
	return &n
}

// prefixAttrs returns attrs with prefix prepended to each key.
func prefixAttrs(prefix string, attrs []slog.Attr) []slog.Attr {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		if a.Key != "" {
			a.Key = prefix + a.Key
		} else if prefix != "" && a.Value.Kind() == slog.KindGroup {
			// Inline group: members need the prefix.
			a = slog.Attr{Key: trimDot(prefix), Value: a.Value}
		}
		out[i] = a
	}
	return out
}

func trimDot(s string) string {
	if s != "" && s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

// _ ensures excludeHandler implements slog.Handler.
var _ slog.Handler = (*excludeHandler)(nil)
