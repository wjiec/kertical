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

	conn *nftables.Conn
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
	err := rw.withOpenConn(func(conn *nftables.Conn) error {
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
	err := rw.withOpenConn(func(conn *nftables.Conn) error {
		for i, table := range rw.tables {
			if table.Name == name {
				rw.tables = append(rw.tables[:i], rw.tables[i+1:]...)
				conn.DelTable(&nftables.Table{Name: name, Family: rw.family})
				break
			}
		}
		return nil
	})
	return errors.Wrapf(err, "failed to delete table: %q", name)
}

// AddChain creates a new chain in the specified table.
//
// If hook is non-nil, it creates a base chain, otherwise a regular chain.
func (rw *readWriter) AddChain(table string, name string, hook *nftables.ChainHook) error {
	err := rw.withOpenConn(func(conn *nftables.Conn) error {
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
func (rw *readWriter) DeleteChain(table string, chain string) error {
	err := rw.withOpenConn(func(conn *nftables.Conn) error {
		for i, elem := range rw.chains {
			if elem.Table.Name == table && elem.Name == chain {
				rw.chains = append(rw.chains[:i], rw.chains[i+1:]...)
				conn.DelChain(&nftables.Chain{
					Name:  chain,
					Table: &nftables.Table{Name: table, Family: rw.family},
				})
				break
			}
		}
		return nil
	})
	return errors.Wrapf(err, "failed to delete chain: %q", chain)
}

// AddRule creates a new rule in the specified table and chain with the given expressions.
func (rw *readWriter) AddRule(table string, chain string, expr []expr.Any, first bool, comment []byte) error {
	err := rw.withOpenConn(func(conn *nftables.Conn) error {
		var addRule = conn.AddRule
		if first {
			addRule = conn.InsertRule
		}

		addRule(&nftables.Rule{
			Table:    &nftables.Table{Name: table, Family: rw.family},
			Chain:    &nftables.Chain{Name: chain},
			Exprs:    expr,
			UserData: comment,
		})
		return nil
	})
	return errors.Wrapf(err, "failed to add rule in table %q chain %q", table, chain)
}

// DeleteRule removes a rule identified by its handle from the specified table and chain.
func (rw *readWriter) DeleteRule(table string, chain string, handle uint64) error {
	err := rw.withOpenConn(func(conn *nftables.Conn) error {
		for i, rule := range rw.rules {
			if rule.Handle == handle {
				rw.rules = append(rw.rules[:i], rw.rules[i+1:]...)
				return conn.DelRule(&nftables.Rule{
					Table:  &nftables.Table{Name: table, Family: rw.family},
					Chain:  &nftables.Chain{Name: chain},
					Handle: handle,
				})
			}
		}
		return nil
	})
	return errors.Wrapf(err, "failed to delete rule in table %q chain %q", table, chain)
}

// AddSet creates a new set in the specified table.
func (rw *readWriter) AddSet(table string, setId uint32, setName string, key, value nftables.SetDatatype, kvPairs []nftables.SetElement, comment string) error {
	err := rw.withOpenConn(func(conn *nftables.Conn) error {
		return conn.AddSet(&nftables.Set{
			Table:     &nftables.Table{Name: table, Family: rw.family},
			ID:        setId,
			Name:      setName,
			IsMap:     len(value.Name) != 0,
			KeyType:   key,
			DataType:  value,
			Anonymous: true,
			Constant:  true,
			Comment:   comment,
		}, kvPairs)
	})
	return errors.Wrapf(err, "failed to add anonymous set in table %q", table)
}

// refresh updates the internal state by fetching the current chains, sets and rules from nftables.
func (rw *readWriter) refresh() error {
	return rw.withOpenConn(func(conn *nftables.Conn) (err error) {
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

// withOpenConn creates a new nftables connection and executes the provided action function with it.
func (rw *readWriter) withOpenConn(action func(*nftables.Conn) error) (err error) {
	// In the nftables package, each command that needs to be sent is first stored
	// in the messages field of the netlink connection object, and then submitted
	// together during Flush. Therefore, we need to create a new connection each
	// time to prevent asynchronous or concurrency issues.
	if rw.conn == nil {
		rw.conn, err = nftables.New()
		if err != nil {
			return errors.Wrap(err, "failed to open netlink connection for nftables")
		}
	}

	return action(rw.conn)
}

// Flush sends all buffered commands in a single batch to nftables.
func (rw *readWriter) Flush() error {
	defer func() { rw.conn = nil }()
	return errors.Wrap(rw.conn.Flush(), "failed to flush commands to nftables")
}
