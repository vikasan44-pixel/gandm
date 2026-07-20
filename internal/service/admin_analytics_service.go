package service

import (
	"context"
	"time"
)

// AnalyticsPeriodDays: 0 = за всё время.
type Analytics struct {
	PeriodDays int `json:"period_days"`

	NewUsers       int `json:"new_users"`
	CargoSubmitted int `json:"cargo_submitted"`
	DealsMatched   int `json:"deals_matched"`
	Verified       int `json:"verified"`

	RegistrationsByDay []DayCount       `json:"registrations_by_day"`
	ParticipantTypes   []TypeCount      `json:"participant_types"`
	TopDirections      []DirectionCount `json:"top_directions"`
}

type DayCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type DirectionCount struct {
	OriginLabel      string `json:"origin_label"`
	DestinationLabel string `json:"destination_label"`
	Count            int    `json:"count"`
}

// Analytics aggregates the admin analytics page (ТЗ §19.7) straight from
// the DB — no separate tracking tables. days <= 0 means "всё время".
func (s *AdminService) Analytics(ctx context.Context, days int) (*Analytics, error) {
	var since time.Time
	if days > 0 {
		since = time.Now().AddDate(0, 0, -days)
	} // else zero time — matches everything

	a := &Analytics{PeriodDays: days}

	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE created_at >= $1`, since).Scan(&a.NewUsers); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM cargo_requests WHERE created_at >= $1`, since).Scan(&a.CargoSubmitted); err != nil {
		return nil, err
	}
	// «Сделок» = закрытые конкурсы: подобранные одиночные грузы + подобранные
	// консолидации за период (по времени создания заявки — приближение, у
	// сделки нет собственного timestamp).
	if err := s.db.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM cargo_requests WHERE status = 'matched' AND created_at >= $1)
		     + (SELECT count(*) FROM consolidated_requests WHERE status = 'matched' AND created_at >= $1)`,
		since).Scan(&a.DealsMatched); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM verification_requests WHERE status = 'approved' AND reviewed_at >= $1`,
		since).Scan(&a.Verified); err != nil {
		return nil, err
	}

	// График регистраций: для «всего времени» показываем последние 30 дней,
	// иначе столбцы нечитаемы.
	chartSince := since
	if days <= 0 {
		chartSince = time.Now().AddDate(0, 0, -30)
	}
	rows, err := s.db.Query(ctx, `
		SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD'), count(*)
		FROM users WHERE created_at >= $1
		GROUP BY 1 ORDER BY 1`, chartSince)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	a.RegistrationsByDay = make([]DayCount, 0)
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return nil, err
		}
		a.RegistrationsByDay = append(a.RegistrationsByDay, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Service categories come from enabled participant tools, not the retired
	// one-role participant_type column. A multi-service company is correctly
	// represented in each service it actually provides.
	typeRows, err := s.db.Query(ctx, `
		WITH service_users AS (
			SELECT DISTINCT ut.user_id, CASE
				WHEN t.key = 'manage_warehouse_slots' THEN 'warehouse'
				WHEN t.key IN ('manage_fleet','receive_cargo_by_route','submit_offer') THEN 'carrier'
				WHEN t.key = 'manage_customs_docs' THEN 'customs_rep'
			END AS service_type
			FROM user_tools ut JOIN tools t ON t.id = ut.tool_id
			WHERE t.key IN ('manage_warehouse_slots','manage_fleet','receive_cargo_by_route','submit_offer','manage_customs_docs')
		), categories AS (
			SELECT user_id, service_type FROM service_users WHERE service_type IS NOT NULL
			UNION ALL
			SELECT u.id, 'client' FROM users u
			WHERE NOT EXISTS (SELECT 1 FROM service_users su WHERE su.user_id = u.id)
		)
		SELECT service_type, count(DISTINCT user_id)
		FROM categories GROUP BY service_type ORDER BY 2 DESC`)
	if err != nil {
		return nil, err
	}
	defer typeRows.Close()
	a.ParticipantTypes = make([]TypeCount, 0)
	for typeRows.Next() {
		var tc TypeCount
		if err := typeRows.Scan(&tc.Type, &tc.Count); err != nil {
			return nil, err
		}
		a.ParticipantTypes = append(a.ParticipantTypes, tc)
	}
	if err := typeRows.Err(); err != nil {
		return nil, err
	}

	dirRows, err := s.db.Query(ctx, `
		SELECT origin_label, destination_label, count(*)
		FROM cargo_requests WHERE created_at >= $1
		GROUP BY 1, 2 ORDER BY 3 DESC LIMIT 5`, since)
	if err != nil {
		return nil, err
	}
	defer dirRows.Close()
	a.TopDirections = make([]DirectionCount, 0, 5)
	for dirRows.Next() {
		var dc DirectionCount
		if err := dirRows.Scan(&dc.OriginLabel, &dc.DestinationLabel, &dc.Count); err != nil {
			return nil, err
		}
		a.TopDirections = append(a.TopDirections, dc)
	}
	return a, dirRows.Err()
}
