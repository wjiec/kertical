//go:build linux

package nftables

import (
	"iter"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/pkg/errors"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/derive"
)

// setWithElement represents a nftables set along with its elements.
type setWithElement struct {
	set      *nftables.Set
	elements []nftables.SetElement // collection of elements contained in the set
}

// readWriter providing access to and manipulation of nftables chains, sets and rules.
type readWriter struct {
	name   string
	family nftables.TableFamily

	tables []*nftables.Table
	chains []*nftables.Chain
	rules  []*nftables.Rule
	sets   []*setWithElement
}

// newReadWriter creates a new readWriter instance and populates it with
// the current state of nftables from the system.
func newReadWriter(name string, family nftables.TableFamily) (*readWriter, error) {
	rw := &readWriter{name: name, family: family}
	if err := rw.refresh(); err != nil {
		return nil, errors.Wrap(err, "failed to refresh chains and rules from nftables")
	}
	return rw, nil
}

// Chains returns a sequence of chains filtered by the specified table family.
func (rw *readWriter) Chains() iter.Seq[*nftables.Chain] {
	return func(yield func(*nftables.Chain) bool) {
		for _, chain := range rw.chains {
			if chain.Table.Family == rw.family {
				if !yield(chain) {
					return
				}
			}
		}
	}
}

// Rules returns a sequence of rules filtered by the specified table name.
func (rw *readWriter) Rules(table string) iter.Seq[*nftables.Rule] {
	return func(yield func(*nftables.Rule) bool) {
		for _, rule := range rw.rules {
			if rule.Table.Name == table && rule.Table.Family == rw.family {
				if !yield(rule) {
					return
				}
			}
		}
	}
}

// Sets returns a sequence of all sets in the nftables table.
func (rw *readWriter) Sets(table string) iter.Seq[*nftables.Set] {
	return func(yield func(*nftables.Set) bool) {
		for _, elem := range rw.sets {
			if elem.set.Table.Name == table {
				if !yield(elem.set) {
					return
				}
			}
		}
	}
}

// Elements returns a sequence of elements in the specified table and set.
func (rw *readWriter) Elements(table string, set string) iter.Seq[*nftables.SetElement] {
	return func(yield func(*nftables.SetElement) bool) {
		for _, elem := range rw.sets {
			if elem.set.Table.Name == table && elem.set.Name == set {
				for _, element := range elem.elements {
					if !yield(&element) {
						return
					}
				}
			}
		}
	}
}

// AddTable creates a new table with the specified name.
func (rw *readWriter) AddTable(name string) error {
	err := withOpenConn(func(conn *nftables.Conn) error {
		conn.AddTable(&nftables.Table{
			Name:   name,
			Family: rw.family,
		})
		return nil
	})
	return errors.Wrapf(err, "failed to add table: %q", name)
}

// DeleteTable removes a table from the specified name.
func (rw *readWriter) DeleteTable(name string) error {
	err := rw.withOpenConnAndRefresh(func(conn *nftables.Conn) error {
		conn.DelTable(&nftables.Table{Name: name, Family: rw.family})
		return nil
	})
	return errors.Wrapf(err, "failed to delete table: %q", name)
}

// AddChain creates a new chain in the specified table.
//
// If hook is non-nil, it creates a base chain, otherwise a regular chain.
func (rw *readWriter) AddChain(table string, name string, hook *nftables.ChainHook) error {
	err := withOpenConn(func(conn *nftables.Conn) error {
		chain := &nftables.Chain{
			Name:    name,
			Table:   &nftables.Table{Name: table, Family: rw.family},
			Hooknum: hook,
		}
		if chain.Hooknum != nil {
			chain.Priority = derive.ChainPriority(chain.Hooknum)
			chain.Type = derive.ChainType(chain.Hooknum)
			chain.Policy = derive.ChainPolicy(chain.Hooknum)
		}

		conn.AddChain(chain)
		return nil
	})
	return errors.Wrapf(err, "failed to add chain: %q", name)
}

// DeleteChain removes a chain from the specified table.
func (rw *readWriter) DeleteChain(table string, name string) error {
	err := rw.withOpenConnAndRefresh(func(conn *nftables.Conn) error {
		conn.DelChain(&nftables.Chain{
			Name:  name,
			Table: &nftables.Table{Name: table, Family: rw.family},
		})
		return nil
	})
	return errors.Wrapf(err, "failed to delete chain: %q", name)
}

// AddRule creates a new rule in the specified table and chain with the given expressions.
func (rw *readWriter) AddRule(table string, chain string, expr []expr.Any, first bool, comment string) error {
	err := withOpenConn(func(conn *nftables.Conn) error {
		var addRule = conn.AddRule
		if first {
			addRule = conn.InsertRule
		}

		addRule(&nftables.Rule{
			Table:    &nftables.Table{Name: table, Family: rw.family},
			Chain:    &nftables.Chain{Name: chain},
			Exprs:    expr,
			UserData: marshalUserComment(comment),
		})
		return nil
	})
	return errors.Wrapf(err, "failed to add rule in table %q chain %q", table, chain)
}

// DeleteRule removes a rule identified by its handle from the specified table and chain.
func (rw *readWriter) DeleteRule(table string, chain string, handler uint64) error {
	err := rw.withOpenConnAndRefresh(func(conn *nftables.Conn) error {
		return conn.DelRule(&nftables.Rule{
			Table:  &nftables.Table{Name: table, Family: rw.family},
			Chain:  &nftables.Chain{Name: chain},
			Handle: handler,
		})
	})
	return errors.Wrapf(err, "failed to delete rule in table %q chain %q", table, chain)
}

// AddSet creates a new set in the specified table.
func (rw *readWriter) AddSet(table, set string, key, value nftables.SetDatatype, comment string) error {
	err := withOpenConn(func(conn *nftables.Conn) error {
		return conn.AddSet(&nftables.Set{
			Table:        &nftables.Table{Name: table, Family: rw.family},
			Name:         set,
			IsMap:        len(value.Name) != 0,
			KeyType:      key,
			DataType:     value,
			KeyByteOrder: nil,
			Comment:      comment,
		}, nil)
	})
	return errors.Wrapf(err, "failed to add set %q in table %q", set, table)
}

// DeleteSet removes a set from the specified table.
func (rw *readWriter) DeleteSet(table, set string) error {
	err := rw.withOpenConnAndRefresh(func(conn *nftables.Conn) error {
		conn.DelSet(&nftables.Set{
			Table: &nftables.Table{Name: table, Family: rw.family},
			Name:  set,
		})
		return nil
	})
	return errors.Wrapf(err, "failed to delete set %q in table %q", set, table)
}

// AddElement adds an element to a set in the specified table.
//
// For regular sets, only key is used. For maps (dictionaries), both key and value are used.
func (rw *readWriter) AddElement(table, set string, key, value []byte, comment string) error {
	err := withOpenConn(func(conn *nftables.Conn) error {
		return conn.SetAddElements(&nftables.Set{
			Table: &nftables.Table{Name: table, Family: rw.family},
			Name:  set,
		}, []nftables.SetElement{
			{
				Key:     key,
				Val:     value,
				Comment: comment,
			},
		})
	})
	return errors.Wrapf(err, "failed to add element in table %q set %q", table, set)
}

// DeleteElement removes an element from a set in the specified table.
func (rw *readWriter) DeleteElement(table, set string, key, value []byte, comment string) error {
	err := rw.withOpenConnAndRefresh(func(conn *nftables.Conn) error {
		return conn.SetDeleteElements(&nftables.Set{
			Table: &nftables.Table{Name: table, Family: rw.family},
			Name:  set,
		}, []nftables.SetElement{
			{
				Key:     key,
				Val:     value,
				Comment: comment,
			},
		})
	})
	return errors.Wrapf(err, "failed to delete element in table %q set %q", table, set)
}

// withOpenConnAndRefresh performs an action with an open nftables connection
// and refreshes the chains, sets and rules afterward.
func (rw *readWriter) withOpenConnAndRefresh(action func(*nftables.Conn) error) error {
	if err := withOpenConn(action); err != nil {
		return err
	}
	return rw.refresh()
}

// refresh updates the internal state by fetching the current chains, sets and rules from nftables.
func (rw *readWriter) refresh() error {
	return withOpenConn(func(conn *nftables.Conn) (err error) {
		if rw.tables, err = conn.ListTablesOfFamily(rw.family); err != nil {
			return err
		}
		if rw.chains, err = conn.ListChains(); err != nil {
			return err
		}

		rw.rules = rw.rules[:0]
		for _, chain := range rw.chains {
			if chain.Hooknum != nil || strings.HasPrefix(chain.Name, rw.name) {
				rules, err := conn.GetRules(chain.Table, chain)
				if err != nil {
					return err
				}
				rw.rules = append(rw.rules, rules...)
			}
		}

		rw.sets = rw.sets[:0]
		for _, table := range rw.tables {
			sets, err := conn.GetSets(table)
			if err != nil {
				return err
			}

			for _, set := range sets {
				elements, err := conn.GetSetElements(set)
				if err != nil {
					return err
				}
				rw.sets = append(rw.sets, &setWithElement{
					set:      set,
					elements: elements,
				})
			}
		}

		return
	})
}
