//go:build linux

package mutation

import (
	"iter"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/pkg/errors"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/condition"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/derive"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/mutation/filter"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/mutation/predicate"
)

// Mutation defines an interface for creating and removing objects and rules in nftables.
type Mutation[R any, W any] interface {
	// Present ensures the mutation is applied in the current state.
	//
	// Returns a function that makes the necessary changes if needed.
	Present(rd R) func(wr W) error

	// CleanUp reverses the mutation, removing any changes it made.
	//
	// Returns a function that cleans up the changes.
	CleanUp(rd R) func(wr W) error
}

// runPresent applies a series of mutations to ensure they are present in the current state.
//
// It processes each mutation in sequence and stops if any returns an error.
func runPresent[R any, W any, M Mutation[R, W]](rd R, wr W, mutations ...M) error {
	for _, mutation := range mutations {
		if f := mutation.Present(rd); f != nil {
			if err := f(wr); err != nil {
				return err
			}
		}
	}
	return nil
}

// runCleanUp reverses a series of mutations, removing any changes they made.
//
// It processes each mutation in sequence and stops if any returns an error.
func runCleanUp[R any, W any, M Mutation[R, W]](rd R, wr W, mutations ...M) error {
	for i := len(mutations) - 1; i >= 0; i-- {
		if f := mutations[i].CleanUp(rd); f != nil {
			if err := f(wr); err != nil {
				return err
			}
		}
	}
	return nil
}

// TableReader provides access to chains and rules within nftables tables.
type TableReader interface {
	// Chains returns a sequence of all chains in the nftables configuration.
	Chains() iter.Seq[*nftables.Chain]

	// Rules returns a sequence of all rules in the nftables configuration.
	Rules(table string) iter.Seq[*nftables.Rule]
}

// TableWriter provides methods to modify nftables tables, chains, and rules.
type TableWriter interface {
	// AddTable creates a new table with the specified name.
	AddTable(name string) error

	// AddChain creates a new chain in the specified table.
	AddChain(table string, name string, hook *nftables.ChainHook) error

	// DeleteChain removes a chain from the specified table.
	DeleteChain(table string, name string) error

	// AddRule creates a new rule in the specified table and chain.
	AddRule(table string, chain string, expr []expr.Any, first bool, comment string) error

	// DeleteRule removes a rule from the specified table chain.
	DeleteRule(table string, chain string, handler uint64) error
}

// TableMutation defines an interface for creating and removing
// nftables tables and their contents.
type TableMutation interface {
	Mutation[TableReader, TableWriter]

	// AddChain adds one or more chain mutations to the table.
	AddChain(chains ...ChainMutation) TableMutation
}

// Table creates a new TableMutation for a specific table and hook.
//
// It ensures the table exists and configures the specified chains within it.
func Table(name string, hook *nftables.ChainHook) TableMutation {
	return &table{name: name, hook: hook}
}

// table implements the TableMutation interface.
type table struct {
	name   string
	hook   *nftables.ChainHook
	chains []ChainMutation
}

// Present ensures the table exists and contains the required chains and rules.
//
// If no table with the specified hook exists, it creates one.
// If multiple tables with the specified hook exist, it updates all of them.
func (t *table) Present(tr TableReader) func(TableWriter) error {
	groups := make(map[string][]*nftables.Chain)
	for elem := range predicate.Filter(tr.Chains(), filter.ChainHook(t.hook)) {
		groups[elem.Table.Name] = append(groups[elem.Table.Name], elem)
	}

	return func(tw TableWriter) error {
		if len(groups) == 0 {
			// Find a table name that matches the most chains with the required chain type.
			name := mostFrequencyTable(tr, derive.ChainType(t.hook), t.name)
			if err := tw.AddTable(name); err != nil {
				return errors.Wrap(err, "failed to add table to nftables")
			}
			groups[name] = append(groups[name], &nftables.Chain{})
		}

		// For security priority and fast failure considerations, traffic needs to be allowed
		// by all chains with the same hook to pass through, so we need to create rule chains
		// in each table.
		for name := range groups {
			trw := &chainReadWriter{tr: tr, tw: tw, table: name, hook: t.hook}
			if err := runPresent[ChainReader, ChainWriter](trw, trw, t.chains...); err != nil {
				return err
			}
		}
		return nil
	}
}

// CleanUp removes all chains and rules that were added to tables with the specified hook.
//
// It doesn't remove the tables themselves, just the chains and rules this mutation added.
func (t *table) CleanUp(tr TableReader) func(TableWriter) error {
	groups := make(map[string][]*nftables.Chain)
	for elem := range predicate.Filter(tr.Chains(), filter.ChainHook(t.hook)) {
		groups[elem.Table.Name] = append(groups[elem.Table.Name], elem)
	}

	return func(tw TableWriter) error {
		for name := range groups {
			crw := &chainReadWriter{tr: tr, tw: tw, table: name, hook: t.hook}
			if err := runCleanUp[ChainReader, ChainWriter](crw, crw, t.chains...); err != nil {
				return err
			}
		}
		return nil
	}
}

// AddChain adds one or more chain mutations to this table mutation.
func (t *table) AddChain(chains ...ChainMutation) TableMutation {
	t.chains = append(t.chains, chains...)
	return t
}

// mostFrequencyTable finds the table name that contains the most chains of the specified type.
//
// If there is a tie or no chains of the specified type are found, it returns the fallback name.
func mostFrequencyTable(tr TableReader, ct nftables.ChainType, fallback string) string {
	name, count := fallback, 1
	for elem := range tr.Chains() {
		if elem.Type == ct {
			if elem.Table.Name == name {
				count++
			} else if count--; count < 0 {
				name = elem.Table.Name
				count = 1
			}
		}
	}
	return name
}

// chainReadWriter implements both ChainReader and ChainWriter interfaces.
type chainReadWriter struct {
	tr TableReader
	tw TableWriter

	table string
	hook  *nftables.ChainHook
}

// Chains returns all chains in the table family.
func (trw *chainReadWriter) Chains() iter.Seq[*nftables.Chain] {
	return trw.tr.Chains()
}

// HookedChains returns all chains that have hooks attached.
func (trw *chainReadWriter) HookedChains() iter.Seq[*nftables.Chain] {
	return predicate.Filter(trw.Chains(), filter.ChainHook(trw.hook))
}

// Rules returns all rules in the specified table and family.
func (trw *chainReadWriter) Rules() iter.Seq[*nftables.Rule] {
	return trw.tr.Rules(trw.table)
}

// AddBaseChain adds a base chain to the table.
func (trw *chainReadWriter) AddBaseChain(name string) error {
	return trw.tw.AddChain(trw.table, name, trw.hook)
}

// AddRegularChain adds a regular chain to the table.
func (trw *chainReadWriter) AddRegularChain(name string) error {
	return trw.tw.AddChain(trw.table, name, nil)
}

// DeleteChain removes a chain from the table.
func (trw *chainReadWriter) DeleteChain(name string) error {
	return trw.tw.DeleteChain(trw.table, name)
}

// AddRule adds a rule to a chain in the table.
func (trw *chainReadWriter) AddRule(chain string, expr []expr.Any, first bool, comment string) error {
	return trw.tw.AddRule(trw.table, chain, expr, first, comment)
}

// DeleteRule removes a rule from a chain in the table.
func (trw *chainReadWriter) DeleteRule(chain string, handle uint64) error {
	return trw.tw.DeleteRule(trw.table, chain, handle)
}

// ChainReader provides access to the current nftables chains and their rules.
type ChainReader interface {
	// Chains returns a sequence of all chains in the nftables configuration.
	Chains() iter.Seq[*nftables.Chain]

	// HookedChains returns a sequence of all chains that have hooks attached.
	HookedChains() iter.Seq[*nftables.Chain]

	// Rules returns a sequence of all rules in the nftables configuration.
	Rules() iter.Seq[*nftables.Rule]
}

// ChainWriter provides methods to modify nftables chains and their rules.
type ChainWriter interface {
	// AddBaseChain creates a new base chain with the given name.
	AddBaseChain(name string) error

	// AddRegularChain creates a new regular chain with the given name.
	AddRegularChain(name string) error

	// DeleteChain removes a chain with the specified name.
	DeleteChain(name string) error

	// AddRule creates a new rule in the specified chain with the given expressions.
	AddRule(chain string, expr []expr.Any, first bool, comment string) error

	// DeleteRule removes a rule identified by its chain and handle.
	DeleteRule(chain string, handle uint64) error
}

// ChainRuleCleanUpConditionFunc is a function type that evaluates whether a chain's rules
// should be cleaned up based on the current state of the nftables configuration.
type ChainRuleCleanUpConditionFunc func(ChainReader) (bool, error)

// ChainMutation is an interface for evaluating and manipulating nftables chains.
type ChainMutation interface {
	Mutation[ChainReader, ChainWriter]

	// AddRule adds one or more rule mutations to the chain.
	AddRule(rules ...RuleMutation) ChainMutation
}

// Chain creates a ChainMutation for a chain with the specified name.
func Chain(name string) ChainMutation {
	return &chain{name: name}
}

// chain implements the ChainMutation interface for any type of chain.
//
// It doesn't create the chain itself, but only manages rules within an existing chain.
type chain struct {
	name  string
	rules []RuleMutation
}

// Present ensures the rules in this chain mutation are properly applied
// to the named chain. It doesn't create the chain, just manages its rules.
func (c *chain) Present(cr ChainReader) func(ChainWriter) error {
	return func(cw ChainWriter) error {
		rrw := &ruleReadWriter{cr: cr, cw: cw, chain: c.name}
		return c.present(rrw, rrw)
	}
}

// CleanUp removes the rules in this chain mutation from the named chain.
// It doesn't remove the chain itself, just the rules this mutation added.
func (c *chain) CleanUp(cr ChainReader) func(ChainWriter) error {
	return func(cw ChainWriter) error {
		rrw := &ruleReadWriter{cr: cr, cw: cw, chain: c.name}
		return c.cleanUp(rrw, rrw)
	}
}

// AddRule adds rule mutations to this chain mutation.
func (c *chain) AddRule(rules ...RuleMutation) ChainMutation {
	c.rules = append(c.rules, rules...)
	return c
}

// present presents all the rule associated with this chain.
func (c *chain) present(rr RuleReader, rw RuleWriter) error {
	return runPresent(rr, rw, c.rules...)
}

// cleanUp removes all the rules associated with this chain.
func (c *chain) cleanUp(rr RuleReader, rw RuleWriter) error {
	return runCleanUp(rr, rw, c.rules...)
}

// ShouldCleanUpAtChainEmpty returns a cleanup condition function that checks if a specified chain is empty.
//
// The returned function, when evaluated, returns true (allowing cleanup) if the chain has no rules,
// or false (preventing cleanup) if the chain still contains rules.
func ShouldCleanUpAtChainEmpty(chain string) ChainRuleCleanUpConditionFunc {
	return func(cr ChainReader) (bool, error) {
		for elem := range cr.Rules() {
			if elem.Chain.Name == chain {
				return false, nil
			}
		}
		return true, nil
	}
}

// BaseChain creates a ChainMutation that ensures a base chain with
// the specified name exists and contains the necessary rules.
func BaseChain(name string) ChainMutation {
	return &baseChain{name: name}
}

// baseChain implements the ChainMutation interface for base chains.
type baseChain struct {
	chain
	name string
}

// Present ensures the base chain exists and contains the required rules.
//
// If no hooked chains exist, it creates one with the specified name.
// If multiple hooked chains exist, it ensures all rules are present in each chain.
func (bc *baseChain) Present(cr ChainReader) func(ChainWriter) error {
	return func(cw ChainWriter) error {
		// If no chain with a hook exists, we need to create one and persist our rules in it.
		if !predicate.Any(cr.HookedChains()) {
			if err := cw.AddBaseChain(bc.name); err != nil {
				return err
			}

			rrw := &ruleReadWriter{cr: cr, cw: cw, chain: bc.name}
			return bc.chain.present(rrw, rrw)
		}

		// If multiple chains with the same hook exist, we need to persist our rules in all of them.
		for elem := range cr.HookedChains() {
			rrw := &ruleReadWriter{cr: cr, cw: cw, chain: elem.Name}
			if err := bc.chain.present(rrw, rrw); err != nil {
				return err
			}
		}

		return nil
	}
}

// CleanUp removes all rules that were added to matching chains.
//
// It doesn't remove the chains themselves, just the rules this mutation added.
func (bc *baseChain) CleanUp(cr ChainReader) func(ChainWriter) error {
	return func(cw ChainWriter) error {
		// We need to remove all rules we added from matching chains
		for elem := range cr.HookedChains() {
			rrw := &ruleReadWriter{cr: cr, cw: cw, chain: elem.Name}
			if err := bc.chain.cleanUp(rrw, rrw); err != nil {
				return err
			}
		}
		return nil
	}
}

// AddRule adds rule mutations to this chain mutation.
func (bc *baseChain) AddRule(rules ...RuleMutation) ChainMutation {
	bc.rules = append(bc.rules, rules...)
	return bc
}

// RegularChain creates an ChainMutation that ensures a regular chain with the specified name exists.
//
// Regular chains don't have hooks and are used for organization and rule grouping.
// They can be targeted by jump or goto operations from other chains.
func RegularChain(name string) ChainMutation {
	return &regularChain{name: name}
}

// regularChain implements the ChainMutation interface for regular chains.
type regularChain struct {
	chain
	name string // The name of the regular chain
}

// Present ensures the regular chain exists and contains the required rules.
//
// If a chain with the same name exists but has a hook (is a base chain),
// it replaces it with a regular chain.
func (rc *regularChain) Present(cr ChainReader) func(ChainWriter) error {
	found, _ := predicate.First(cr.Chains(), filter.ChainName(rc.name))
	return func(cw ChainWriter) error {
		if found != nil && found.Hooknum != nil {
			if err := cw.DeleteChain(found.Name); err != nil {
				return err
			}
		}

		// Creating a chain that already exists is allowed.
		if err := cw.AddRegularChain(rc.name); err != nil {
			return err
		}

		rrw := &ruleReadWriter{cr: cr, cw: cw, chain: rc.name}
		return rc.chain.present(rrw, rrw)
	}
}

// CleanUp removes all rules that were added to this regular chain and then
// removes the chain itself if empty.
func (rc *regularChain) CleanUp(cr ChainReader) func(ChainWriter) error {
	return func(cw ChainWriter) error {
		rrw := &ruleReadWriter{cr: cr, cw: cw, chain: rc.name}
		if err := rc.chain.cleanUp(rrw, rrw); err != nil {
			return err
		}

		for elem := range cr.Rules() {
			if elem.Chain.Name == rc.name {
				return nil
			}
		}
		return cw.DeleteChain(rc.name) // todo here
	}
}

// AddRule adds rule mutations to this chain mutation.
func (rc *regularChain) AddRule(rules ...RuleMutation) ChainMutation {
	rc.rules = append(rc.rules, rules...)
	return rc
}

// ruleReadWriter implements the RuleReader and RuleWriter interfaces
// for a specific chain, filtering rules by chain name and directing
// operations to that chain.
type ruleReadWriter struct {
	cr    ChainReader
	cw    ChainWriter
	chain string
}

// Rules returns only the rules that belong to this chain.
func (crw *ruleReadWriter) Rules() iter.Seq[*nftables.Rule] {
	return predicate.Filter(crw.cr.Rules(), filter.RuleChainName(crw.chain))
}

// AddRule adds a rule to this specific chain.
func (crw *ruleReadWriter) AddRule(expr []expr.Any, first bool, comment string) error {
	return crw.cw.AddRule(crw.chain, expr, first, comment)
}

// DeleteRule removes a rule from this specific chain.
func (crw *ruleReadWriter) DeleteRule(handler uint64) error {
	return crw.cw.DeleteRule(crw.chain, handler)
}

// RuleReader provides access to the current nftables rules in the nftables chains.
type RuleReader interface {
	// Rules returns a sequence of all rules in the nftables chains.
	Rules() iter.Seq[*nftables.Rule]
}

// RuleWriter provides methods to modify nftables rules.
type RuleWriter interface {
	// AddRule creates a new rule with the given expressions.
	AddRule(expr []expr.Any, first bool, comment string) error

	// DeleteRule removes a rule identified by its handle.
	DeleteRule(handler uint64) error
}

// RuleMutation is a specialized Mutation for working with nftables rules.
type RuleMutation interface {
	Mutation[RuleReader, RuleWriter]

	// First sets whether this rule should be inserted at the beginning of the chain.
	First(first bool) RuleMutation

	// Comment sets a descriptive comment for this rule.
	Comment(comment string) RuleMutation
}

// Rule creates a RuleMutation that ensures a rule with the specified conditions exists.
func Rule(conditions ...condition.Condition) RuleMutation {
	return &rule{expr: condition.Combine(conditions...)}
}

// rule implements the RuleMutation interface for creating and cleaning rules.
type rule struct {
	expr    condition.Condition
	first   bool   // Whether to insert the rule at the beginning of the chain
	comment string // Comment to attach to the rule for identification
}

// Present checks if a rule with the specified expressions already exists.
//
// If exactly one matching rule is found, it does nothing.
// If multiple matching rules exist, it keeps the first one and deletes the duplicates.
// If no matching rule exists, it adds a new one.
func (r *rule) Present(rr RuleReader) func(RuleWriter) error {
	if found, rest := predicate.First(rr.Rules(), filter.RuleExpr(r.expr)); found != nil {
		return func(rw RuleWriter) error {
			for elem := range rest {
				if err := rw.DeleteRule(elem.Handle); err != nil {
					return err
				}
			}
			return nil
		}
	}
	// If the rule doesn't exist, we need to create a new one in the current chain.
	return func(rw RuleWriter) error { return rw.AddRule(r.expr.Build(), r.first, r.comment) }
}

// CleanUp removes any rules that match this rule's expressions.
func (r *rule) CleanUp(rr RuleReader) func(RuleWriter) error {
	return func(rw RuleWriter) error {
		for elem := range predicate.Filter(rr.Rules(), filter.RuleExpr(r.expr)) {
			if err := rw.DeleteRule(elem.Handle); err != nil {
				return err
			}
		}
		return nil
	}
}

// First sets whether this rule should be inserted at the beginning of the chain.
// By default, rules are appended to the end of the chain.
func (r *rule) First(first bool) RuleMutation {
	r.first = first
	return r
}

// Comment sets a descriptive comment for this rule.
func (r *rule) Comment(comment string) RuleMutation {
	r.comment = comment
	return r
}
