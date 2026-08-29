package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	usecases "gitlab.com/URL-shortener4224128/analytics-repository/src/application/use_cases"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/dto"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/infrastructure/gotel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

type AnalyticsController struct {
	getAnalyticsUseCase *usecases.GetAnalyticsByDeviceUseCase
	telemetry gotel.TelemetryProvider
	latencyHistogram otelmetric.Int64Histogram
}

func NewAnalyticsController(getAnalyticsUseCase *usecases.GetAnalyticsByDeviceUseCase, telemetry gotel.TelemetryProvider, latencyHistogram otelmetric.Int64Histogram) *AnalyticsController {
	return &AnalyticsController{
		getAnalyticsUseCase: getAnalyticsUseCase,
		telemetry: telemetry,
		latencyHistogram: latencyHistogram,
	}
}

func (a *AnalyticsController) GetAnalyticsByDevice(w http.ResponseWriter, r *http.Request) {
	before := time.Now()

	defer func() {
		duration := time.Since(before).Milliseconds()
		a.latencyHistogram.Record(r.Context(), duration)
	}()

	ctx, span := a.telemetry.TraceStart(r.Context(), "UrlController.GetAllUrls")
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.Path),
	)

	newCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	page := r.URL.Query().Get("page")
	size := r.URL.Query().Get("size")

	if page == "" {
		page = "1"
	}

	if size == "" {
		size = "10"
	}

	var pageInt, sizeInt int

	if pInt, err := strconv.Atoi(page); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		a.telemetry.LogInfo("Status code: 400, message: invalid page parameter value: ", page)

		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	} else {
		pageInt = pInt
	}

	if zInt, err := strconv.Atoi(size); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		a.telemetry.LogInfo("Status code: 400, message: invalid size parameter value: ", size)

		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	} else {
		sizeInt = zInt
	}

	query := &dto.Query{
		Page: pageInt,
		Size: sizeInt,
	}

	res, err := a.getAnalyticsUseCase.Execute(newCtx, query)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		a.telemetry.LogErrorln("Status code: 500, message: failed to get analytics by device, error: ", err.Error())

		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	a.telemetry.LogInfo("Status code: 200, message: successfully retrieved analytics by device")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(res)
}
