package repo

import (
	"context"
	"sync"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
)

const (
	publicStaticOfferTTL     = 20 * time.Second
	publicStaticOfferTimeout = 10 * time.Second
	// IPCrown documents 60 queries per minute. This single control process can
	// spend up to 20 on public traffic, leaving 40 for checkout, renewal and reconciliation.
	publicStaticOffersPerMinute = 20
)

type staticOfferSpec struct {
	shareMode string
	purposeID int32
}

type staticOfferRefresh struct {
	done     chan struct{}
	snapshot wallet.StaticOfferSnapshot
	err      error
}

type publicStaticOfferCache struct {
	mu        sync.Mutex
	snapshots map[staticOfferSpec]wallet.StaticOfferSnapshot
	refreshes map[staticOfferSpec]*staticOfferRefresh
	requests  [publicStaticOffersPerMinute]time.Time
	next      int
}

func (r *WalletRepository) StaticOffers(ctx context.Context, shareMode string, purposeID int32) (wallet.StaticOfferSnapshot, error) {
	if err := r.requireStaticOfferSale(ctx); err != nil {
		return wallet.StaticOfferSnapshot{}, err
	}
	if shareMode == "" {
		shareMode = "exclusive"
	}
	snapshot, err := r.publicStaticOffers.get(ctx, r.static, staticOfferSpec{shareMode, purposeID})
	if err != nil {
		return wallet.StaticOfferSnapshot{}, err
	}
	if err := RequireProductSale(ctx, r.db, product.Static); err != nil {
		return wallet.StaticOfferSnapshot{}, err
	}
	return snapshot, nil
}

func (c *publicStaticOfferCache) get(ctx context.Context, supplier wallet.StaticSupplier, spec staticOfferSpec) (wallet.StaticOfferSnapshot, error) {
	c.mu.Lock()
	now := time.Now()
	snapshot := c.snapshots[spec]
	if now.Sub(snapshot.QuotedAt) < publicStaticOfferTTL {
		c.mu.Unlock()
		return snapshot, nil
	}
	refresh := c.refreshes[spec]
	if refresh == nil {
		if now.Sub(c.requests[c.next]) < time.Minute {
			c.mu.Unlock()
			return wallet.StaticOfferSnapshot{}, wallet.ErrStaticUnavailable
		}
		c.requests[c.next] = now
		c.next = (c.next + 1) % len(c.requests)
		refresh = &staticOfferRefresh{done: make(chan struct{})}
		if c.refreshes == nil {
			c.refreshes = make(map[staticOfferSpec]*staticOfferRefresh)
			c.snapshots = make(map[staticOfferSpec]wallet.StaticOfferSnapshot)
		}
		c.refreshes[spec] = refresh
		go c.refresh(context.WithoutCancel(ctx), supplier, spec, refresh)
	}
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		return wallet.StaticOfferSnapshot{}, wallet.ErrStaticUnavailable
	case <-refresh.done:
		return refresh.snapshot, refresh.err
	}
}

func (c *publicStaticOfferCache) refresh(ctx context.Context, supplier wallet.StaticSupplier, spec staticOfferSpec, refresh *staticOfferRefresh) {
	ctx, cancel := context.WithTimeout(ctx, publicStaticOfferTimeout)
	defer cancel()
	offers, err := supplier.Offers(ctx, spec.shareMode, spec.purposeID)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		refresh.err = wallet.ErrStaticUnavailable
	} else {
		refresh.snapshot = wallet.StaticOfferSnapshot{Offers: offers, QuotedAt: time.Now()}
		c.snapshots[spec] = refresh.snapshot
	}
	delete(c.refreshes, spec)
	close(refresh.done)
}
