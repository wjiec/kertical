//go:build linux

package derive

import (
	"github.com/google/nftables"
	"k8s.io/utils/ptr"
)

// ChainType determines the appropriate nftables chain type based on the provided hook.
func ChainType(hook *nftables.ChainHook) nftables.ChainType {
	switch hook {
	// In the context of implementing port forwarding with nftables, we perform DNAT in the
	// prerouting chain (handling external traffic), output chain (handling locally generated traffic),
	// and masquerade source addresses in the postrouting chain. Therefore, these three chains need to be
	// derived as NAT type.
	case nftables.ChainHookPrerouting, nftables.ChainHookPostrouting, nftables.ChainHookOutput:
		return nftables.ChainTypeNAT

	default:
		// For chains bound to other hooks, we uniformly process them as filter type.
		// In practice, we'll only handle the forward chain here, which we need to allow
		// traffic to pass through.
		return nftables.ChainTypeFilter
	}
}

// ChainPriority returns the appropriate priority value for a chain based on its hook.
func ChainPriority(hook *nftables.ChainHook) *nftables.ChainPriority {
	switch hook {
	case nftables.ChainHookPrerouting:
		return nftables.ChainPriorityNATDest
	case nftables.ChainHookPostrouting:
		return nftables.ChainPriorityNATSource
	case nftables.ChainHookOutput:
		return ptr.To(nftables.ChainPriority(-100))
	default:
		return nftables.ChainPriorityFilter
	}
}

// ChainPolicy returns the appropriate policy value for a chain based on its hook.
func ChainPolicy(hook *nftables.ChainHook) *nftables.ChainPolicy {
	switch hook {
	case nftables.ChainHookForward:
		return ptr.To(nftables.ChainPolicyDrop)
	default:
		return ptr.To(nftables.ChainPolicyAccept)
	}
}
