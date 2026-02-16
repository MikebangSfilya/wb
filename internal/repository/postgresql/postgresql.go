package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/MikebangSfilya/wb/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Repository struct {
	pool *pgxpool.Pool
	tr   trace.Tracer
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
		tr:   otel.Tracer("postgres"),
	}
}

func (r *Repository) CreateOrder(ctx context.Context, order *model.Order) error {
	const op = "postgresql.CreateOrder"

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qOrder := `
       INSERT INTO orders (
               order_uid, track_number, entry, locale, internal_signature, 
               customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard) 
			   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
       ON CONFLICT (order_uid) DO NOTHING
       `
	t, err := tx.Exec(ctx, qOrder, order.OrderUID, order.TrackNumber, order.Entry, order.Locale, order.InternalSignature,
		order.CustomerID, order.DeliveryService, order.Shardkey, order.SmID, order.DateCreated, order.OofShard)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if t.RowsAffected() == 0 {
		return nil
	}

	qDelivery := `
		INSERT INTO delivery (
			order_uid, name, phone, zip, city, address, region, email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = tx.Exec(ctx, qDelivery,
		order.OrderUID,
		order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip, order.Delivery.City,
		order.Delivery.Address, order.Delivery.Region, order.Delivery.Email,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	qPayment := `
		INSERT INTO payment (
			transaction, order_uid, request_id, currency, provider, amount, 
			payment_dt, bank, delivery_cost, goods_total, custom_fee)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = tx.Exec(ctx, qPayment,
		order.Payment.Transaction, order.OrderUID, order.Payment.RequestID, order.Payment.Currency,
		order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDt, order.Payment.Bank,
		order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee,
	)

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	qItems := `
		INSERT INTO items (
			order_uid, chrt_id, track_number, price, rid, name, sale, 
			size, total_price, nm_id, brand, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	for _, item := range order.Items {
		_, err := tx.Exec(ctx, qItems,
			order.OrderUID,
			item.ChrtID, item.TrackNumber, item.Price, item.Rid, item.Name, item.Sale,
			item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status,
		)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *Repository) GetOrder(ctx context.Context, orderUID string) (*model.Order, error) {
	const op = "postgresql.GetOrder"

	ctx, span := r.tr.Start(ctx, "db.select.orders")
	defer span.End()

	qOrder := `
		SELECT 
			o.order_uid, o.track_number, o.entry, o.locale, o.internal_signature, 
			o.customer_id, o.delivery_service, o.shardkey, o.sm_id, o.date_created, o.oof_shard,
			d.name, d.phone, d.zip, d.city, d.address, d.region, d.email,
			p.transaction, p.request_id, p.currency, p.provider, p.amount, p.payment_dt, 
			p.bank, p.delivery_cost, p.goods_total, p.custom_fee
		FROM orders o
		JOIN delivery d ON o.order_uid = d.order_uid
		JOIN payment p ON o.order_uid = p.order_uid
		WHERE o.order_uid = $1
	`

	var o model.Order

	err := r.pool.QueryRow(ctx, qOrder, orderUID).Scan(
		&o.OrderUID, &o.TrackNumber, &o.Entry, &o.Locale, &o.InternalSignature,
		&o.CustomerID, &o.DeliveryService, &o.Shardkey, &o.SmID, &o.DateCreated, &o.OofShard,
		&o.Delivery.Name, &o.Delivery.Phone, &o.Delivery.Zip, &o.Delivery.City,
		&o.Delivery.Address, &o.Delivery.Region, &o.Delivery.Email,
		&o.Payment.Transaction, &o.Payment.RequestID, &o.Payment.Currency, &o.Payment.Provider,
		&o.Payment.Amount, &o.Payment.PaymentDt, &o.Payment.Bank, &o.Payment.DeliveryCost,
		&o.Payment.GoodsTotal, &o.Payment.CustomFee,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "msg")
		return nil, fmt.Errorf("%s: failed to query o: %w", op, err)
	}

	o.Items = make([]model.Item, 0)

	qItems := `
		SELECT 
			chrt_id, track_number, price, rid, name, sale, size, 
			total_price, nm_id, brand, status
		FROM items
		WHERE order_uid = $1
	`

	rows, err := r.pool.Query(ctx, qItems, orderUID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to query items: %w", op, err)
	}
	defer rows.Close()

	for rows.Next() {
		var i model.Item
		err := rows.Scan(
			&i.ChrtID, &i.TrackNumber, &i.Price, &i.Rid, &i.Name, &i.Sale, &i.Size,
			&i.TotalPrice, &i.NmID, &i.Brand, &i.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to scan item: %w", op, err)
		}
		o.Items = append(o.Items, i)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows iteration: %w", op, err)
	}

	return &o, nil
}
