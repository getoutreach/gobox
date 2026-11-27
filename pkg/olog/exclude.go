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
	"maps"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"

	"go.yaml.in/yaml/v3"
)

// ErrInvalidExclude is returned by [ConfigureFromFile] (and surfaced
// through [PollConfigurationFile]'s callback) when exclude is not a
// list, and for each rule that fails to decode, has an unknown key or
// an invalid regular expression, or sets neither message nor attrs.
// Valid rules are still applied.
var ErrInvalidExclude = errors.New("invalid exclude rule")

// errEmptyExclude is returned for a rule with neither message nor attrs.
var errEmptyExclude = errors.New("must set message or attrs")

// errUnknownExcludeKey is returned for a rule with a key other than
// address, message or attrs.
var errUnknownExcludeKey = errors.New("unknown key")

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
//
//nolint:gochecknoglobals // Why: process-wide logger state
var excludeRules atomic.Pointer[[]compiledRule]

// compileExcludes decodes and compiles the exclude list in node,
// returning the valid rules and an error joining one ErrInvalidExclude
// per rejected rule.
func compileExcludes(node *yaml.Node, path string) ([]compiledRule, error) {
	if node.Kind == 0 || node.Tag == "!!null" {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%w in %s: exclude must be a list", ErrInvalidExclude, path)
	}
	var (
		rules []compiledRule
		errs  []error
	)
	for i, n := range node.Content {
		var (
			c ExcludeConfig
			r compiledRule
		)
		err := checkExcludeKeys(n)
		if err == nil {
			err = n.Decode(&c)
		}
		if err == nil {
			r, err = compileExclude(&c)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: rule %d in %s: %w", ErrInvalidExclude, i, path, err))
			continue
		}
		rules = append(rules, r)
	}
	return rules, errors.Join(errs...)
}

// checkExcludeKeys rejects unknown keys, so a typo such as "adress"
// doesn't widen a rule to every logger.
func checkExcludeKeys(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return nil // Decode reports the shape error
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		switch k := n.Content[i].Value; k {
		case "address", "message", "attrs":
		default:
			return fmt.Errorf("%w %q", errUnknownExcludeKey, k)
		}
	}
	return nil
}

func compileExclude(c *ExcludeConfig) (compiledRule, error) {
	if c.Message == "" && len(c.Attrs) == 0 {
		return compiledRule{}, errEmptyExclude
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

	// bound holds attributes added via WithAttrs.
	bound *boundAttrs

	// prefix is the dotted group path (with trailing dot) applied to
	// attributes.
	prefix string
}

// boundAttrs is one WithAttrs call. It is flattened to dotted keys
// and resolved on first use, then cached, so With stays cheap when no
// attrs rule applies.
type boundAttrs struct {
	parent *boundAttrs
	prefix string
	attrs  []slog.Attr

	once sync.Once
	flat map[string]string
}

// get returns the flattened attributes of b and its ancestors.
func (b *boundAttrs) get() map[string]string {
	if b == nil {
		return nil
	}
	b.once.Do(func() {
		p := b.parent.get()
		b.flat = make(map[string]string, len(p)+len(b.attrs))
		maps.Copy(b.flat, p)
		for _, a := range b.attrs {
			flattenAttr(b.prefix, a, b.flat)
		}
	})
	return b.flat
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

func attrsMatch(want map[string]*regexp.Regexp, got map[string]string) bool {
	for k, re := range want {
		v, ok := got[k]
		if !ok || !re.MatchString(v) {
			return false
		}
	}
	return true
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
	n.bound = &boundAttrs{parent: h.bound, prefix: h.prefix, attrs: attrs}
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
	return slices.Contains(h.addrs, address)
}

// collectAttrs merges bound and record attributes into dotted keys
// with resolved string values. Record attributes win over bound ones.
func (h *excludeHandler) collectAttrs(r *slog.Record) map[string]string {
	bound := h.bound.get()
	out := make(map[string]string, len(bound)+r.NumAttrs())
	maps.Copy(out, bound)
	r.Attrs(func(a slog.Attr) bool {
		flattenAttr(h.prefix, a, out)
		return true
	})
	return out
}

// _ ensures excludeHandler implements slog.Handler.
var _ slog.Handler = (*excludeHandler)(nil)
