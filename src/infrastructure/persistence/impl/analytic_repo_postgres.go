package impl

import (
	"context"
	"database/sql"

	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/dto"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/models"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/ports/out"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/infrastructure/gotel"
)

type AnalyticRepoPostgres struct {
	db *sql.DB
	telemetry gotel.TelemetryProvider
}

func NewAnalyticRepoPostgres(db *sql.DB, telemetry gotel.TelemetryProvider) out.AnalyticRepo {
	return &AnalyticRepoPostgres{
		db: db,
		telemetry: telemetry,
	}
}

func (a *AnalyticRepoPostgres) Save(ctx context.Context, analytic *models.Analytic) error {
	_, err := a.db.ExecContext(
		ctx, 
		"INSERT INTO url_visits (url_id, device_name) VALUES ($1, $2)", 
		analytic.UrlId,
		analytic.DeviceName,
	)

	if err != nil {
		a.telemetry.LogErrorln("Database error on Save: failed to insert analytic, error: ", err.Error())
		return err
	}

	return nil
}

func (a *AnalyticRepoPostgres) GetAnalyticsByDevice(ctx context.Context, query *dto.Query) (*dto.Paginate[*dto.Device], error) {
	var totalRecords int
	countSql := `SELECT COUNT(DISTINCT device_name) FROM url_visits`
	
	if err := a.db.QueryRowContext(ctx, countSql).Scan(&totalRecords); err != nil {
		a.telemetry.LogErrorln("Database error on GetAnalyticsByDevice (count): ", err.Error())
		return nil, err
	}

	querySql := `
		SELECT 
			device_name,
			COUNT(*) AS total_device,
			(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER()) AS percentage
		FROM url_visits
		GROUP BY device_name
		ORDER BY percentage DESC
	`

	var args []any
	if query.Page > 0 && query.Size > 0 {
		offset := query.Size * (query.Page - 1)
		querySql += " LIMIT $1 OFFSET $2"
		args = append(args, query.Size, offset)
	}

	rows, err := a.db.QueryContext(ctx, querySql, args...)
	if err != nil {
		a.telemetry.LogErrorln("Database error on GetAnalyticsByDevice (query): ", err.Error())
		return nil, err
	}
	defer rows.Close()

	res := make([]*dto.Device, 0)
	for rows.Next() {
		var device dto.Device
		if err := rows.Scan(&device.DeviceName, &device.Total, &device.Percentage); err != nil {
			a.telemetry.LogErrorln("Database error on GetAnalyticsByDevice (scan): ", err.Error())
			return nil, err
		}
		res = append(res, &device)
	}

	if err := rows.Err(); err != nil {
		a.telemetry.LogErrorln("Database error on GetAnalyticsByDevice (rows iteration): ", err.Error())
		return nil, err
	}

	lastPage := 0
	if query.Size > 0 && totalRecords > 0 {
		lastPage = (totalRecords + query.Size - 1) / query.Size
	}

	return &dto.Paginate[*dto.Device]{
		Result:      res,
		CurrentPage: query.Page,
		LastPage:    lastPage,
	}, nil
}
