package condition_test

import (
	"net"
	"testing"

	"github.com/google/nftables/expr"
	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/condition"
	"github.com/wjiec/kertical/internal/portforwarding/transport"
)

func TestCombine(t *testing.T) {
	assert.NotNil(t, condition.Combine())

	t.Run("build", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			assert.Empty(t, condition.Combine())
		})

		t.Run("multiple", func(t *testing.T) {
			assert.NotEmpty(t, condition.Combine(
				condition.SourceLocalAddr(),
				condition.Tcp(),
				condition.DestinationPort(38080),
				condition.DestinationNAT(net.ParseIP("172.16.1.1"), 8080),
			))
		})
	})

	t.Run("match", func(t *testing.T) {
		c := condition.Combine(
			condition.SourceLocalAddr(),
			condition.Tcp(),
			condition.DestinationPort(38080),
			condition.DestinationNAT(net.ParseIP("172.16.1.1"), 8080),
		)

		a := c.Build()
		assert.Equal(t, len(a), c.Match(a))
	})

}

func TestCounter(t *testing.T) {
	assert.NotNil(t, condition.Counter())

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.Counter().Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.Counter().Build()
		assert.NotZero(t, condition.Counter().Match(a))

		a[0].(*expr.Counter).Bytes = 1 << 20
		a[0].(*expr.Counter).Packets = 1 << 10
		assert.NotZero(t, condition.Counter().Match(a))
	})
}

func TestTransportProtocol(t *testing.T) {
	assert.NotNil(t, condition.TransportProtocol(transport.TCP))
}

func TestTcp(t *testing.T) {
	assert.NotNil(t, condition.Tcp())

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.Tcp().Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.Tcp().Build()
		assert.NotZero(t, condition.Tcp().Match(a))
	})
}

func TestUdp(t *testing.T) {
	assert.NotNil(t, condition.Udp())

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.Udp().Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.Udp().Build()
		assert.NotZero(t, condition.Udp().Match(a))
	})
}

func TestDestinationPort(t *testing.T) {
	assert.NotNil(t, condition.DestinationPort(8080))

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.DestinationPort(8080).Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.DestinationPort(8080).Build()
		assert.Zero(t, condition.DestinationPort(8081).Match(a))
		assert.NotZero(t, condition.DestinationPort(8080).Match(a))
	})
}

func TestSourcePort(t *testing.T) {
	assert.NotNil(t, condition.SourcePort(8080))

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.SourcePort(8080).Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.SourcePort(8080).Build()
		assert.Zero(t, condition.SourcePort(8081).Match(a))
		assert.NotZero(t, condition.SourcePort(8080).Match(a))
	})
}

func TestDestinationIp(t *testing.T) {
	assert.NotNil(t, condition.DestinationIp(net.ParseIP("172.16.1.1")))

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.DestinationIp(net.ParseIP("172.16.1.1")).Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.DestinationIp(net.ParseIP("172.16.1.1")).Build()
		assert.Zero(t, condition.DestinationIp(net.ParseIP("172.16.1.2")).Match(a))
		assert.NotZero(t, condition.DestinationIp(net.ParseIP("172.16.1.1")).Match(a))
	})
}

func TestDestinationNAT(t *testing.T) {
	assert.NotNil(t, condition.DestinationNAT(net.ParseIP("172.16.1.1"), 8080))

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.DestinationNAT(net.ParseIP("172.16.1.1"), 8080).Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.DestinationNAT(net.ParseIP("172.16.1.1"), 8080).Build()
		assert.Zero(t, condition.DestinationNAT(net.ParseIP("172.16.1.2"), 8080).Match(a))
		assert.Zero(t, condition.DestinationNAT(net.ParseIP("172.16.1.1"), 8081).Match(a))
		assert.NotZero(t, condition.DestinationNAT(net.ParseIP("172.16.1.1"), 8080).Match(a))
	})
}

func TestMasquerade(t *testing.T) {
	assert.NotNil(t, condition.Masquerade())

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.Masquerade().Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.Masquerade().Build()
		assert.NotZero(t, condition.Masquerade().Match(a))
	})
}

func TestSourceLocalAddr(t *testing.T) {
	assert.NotNil(t, condition.SourceLocalAddr())

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.SourceLocalAddr().Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.SourceLocalAddr().Build()
		assert.NotZero(t, condition.SourceLocalAddr().Match(a))
	})
}

func TestTrackingEstablishedRelated(t *testing.T) {
	assert.NotNil(t, condition.TrackingEstablishedRelated())

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.TrackingEstablishedRelated().Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.TrackingEstablishedRelated().Build()
		assert.NotZero(t, condition.TrackingEstablishedRelated().Match(a))
	})
}

func TestSetTrackingMark(t *testing.T) {
	assert.NotNil(t, condition.SetTrackingMark(0xabcd))

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.SetTrackingMark(0xabcd).Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.SetTrackingMark(0xabcd).Build()
		assert.NotZero(t, condition.SetTrackingMark(0xabcd).Match(a))
	})
}

func TestTrackingMark(t *testing.T) {
	assert.NotNil(t, condition.TrackingMark(0xabcd))

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.TrackingMark(0xabcd).Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.TrackingMark(0xabcd).Build()
		assert.NotZero(t, condition.TrackingMark(0xabcd).Match(a))
	})
}

func TestAccept(t *testing.T) {
	assert.NotNil(t, condition.Accept())

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.Accept().Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.Accept().Build()
		assert.Zero(t, condition.Accept().Match([]expr.Any{&expr.Verdict{Kind: expr.VerdictJump}}))
		assert.NotZero(t, condition.Accept().Match(a))
	})
}

func TestJump(t *testing.T) {
	assert.NotNil(t, condition.Jump("another-chain"))

	t.Run("build", func(t *testing.T) {
		assert.NotEmpty(t, condition.Jump("another-chain").Build())
	})

	t.Run("match", func(t *testing.T) {
		a := condition.Jump("another-chain").Build()
		assert.Zero(t, condition.Jump("another-another-chain").Match(a))
		assert.NotZero(t, condition.Jump("another-chain").Match(a))
	})
}
