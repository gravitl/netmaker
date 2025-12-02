package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	ch "github.com/gravitl/netmaker/clickhouse"
	"github.com/gravitl/netmaker/logic"
	proLogic "github.com/gravitl/netmaker/pro/logic"
)

func FlowHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/flows", logic.SecurityCheck(true, http.HandlerFunc(handleListFlows))).Methods(http.MethodGet)
}

const (
	querySelect = `
SELECT
	flow_id, host_id, network_id,
	protocol, src_port, dst_port,
	icmp_type, icmp_code, direction,
	src_ip, src_type, src_entity_id,
	dst_ip, dst_type, dst_entity_id,
	start_ts, end_ts,
	bytes_sent, bytes_recv,
	packets_sent, packets_recv,
	status, version
FROM flows`
	queryOrder = `
ORDER BY version DESC
LIMIT ? OFFSET ?`
)

func handleListFlows(w http.ResponseWriter, r *http.Request) {
	if !proLogic.GetFeatureFlags().EnableFlowLogs {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("flow logs not enabled"), logic.Forbidden))
		return
	}

	q := r.URL.Query()

	var (
		whereParts []string
		args       []any
	)

	// 1. Time filtering (version: UInt64 timestamp in ms)
	fromStr := q.Get("from")
	toStr := q.Get("to")

	if fromStr != "" {
		fromVal, err := strconv.ParseUint(fromStr, 10, 64)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid 'from' timestamp: %v", err), logic.BadReq))
			return
		}
		whereParts = append(whereParts, "version >= ?")
		args = append(args, fromVal)
	}

	if toStr != "" {
		toVal, err := strconv.ParseUint(toStr, 10, 64)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid 'to' timestamp: %v", err), logic.BadReq))
			return
		}
		whereParts = append(whereParts, "version <= ?")
		args = append(args, toVal)
	}

	// 2. Source filters
	srcTypeStr := q.Get("src_type")
	if srcTypeStr != "" {
		whereParts = append(whereParts, "src_type = ?")
		args = append(args, srcTypeStr)
	}

	srcEntity := q.Get("src_entity_id")
	if srcEntity != "" {
		whereParts = append(whereParts, "src_entity_id = ?")
		args = append(args, srcEntity)
	}

	// 3. Destination filters
	dstTypeStr := q.Get("dst_type")
	if dstTypeStr != "" {
		whereParts = append(whereParts, "dst_type = ?")
		args = append(args, dstTypeStr)
	}

	dstEntity := q.Get("dst_entity_id")
	if dstEntity != "" {
		whereParts = append(whereParts, "dst_entity_id = ?")
		args = append(args, dstEntity)
	}

	// 4. Protocol filter
	protoStr := q.Get("protocol")
	if protoStr != "" {
		whereParts = append(whereParts, "protocol = ?")
		args = append(args, protoStr)
	}

	// Pagination
	page := parseIntOrDefault(q.Get("page"), 1)
	perPage := parseIntOrDefault(q.Get("per_page"), 100)
	if perPage > 1000 {
		perPage = 1000
	}
	offset := (page - 1) * perPage

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = "WHERE " + strings.Join(whereParts, " AND ")
	}

	query := querySelect + "\n" + whereSQL + "\n" + queryOrder

	args = append(args, perPage, offset)

	rows, err := ch.FromContext(r.Context()).Query(r.Context(), query, args...)
	if err != nil {
		logic.ReturnErrorResponse(w, r,
			logic.FormatError(fmt.Errorf("error fetching flows: %v", err), logic.Internal))
		return
	}
	defer rows.Close()

	type FlowRow struct {
		FlowID      string    `json:"flow_id"`
		HostID      string    `json:"host_id"`
		NetworkID   string    `json:"network_id"`
		Protocol    uint16    `json:"protocol"`
		SrcPort     uint16    `json:"src_port"`
		DstPort     uint16    `json:"dst_port"`
		ICMPType    uint8     `json:"icmp_type"`
		ICMPCode    uint8     `json:"icmp_code"`
		Direction   uint8     `json:"direction"`
		SrcIP       string    `json:"src_ip"`
		SrcType     uint8     `json:"src_type"`
		SrcEntityID string    `json:"src_entity_id"`
		DstIP       string    `json:"dst_ip"`
		DstType     uint8     `json:"dst_type"`
		DstEntityID string    `json:"dst_entity_id"`
		StartTs     time.Time `json:"start_ts"`
		EndTs       time.Time `json:"end_ts"`
		BytesSent   uint64    `json:"bytes_sent"`
		BytesRecv   uint64    `json:"bytes_recv"`
		PacketsSent uint64    `json:"packets_sent"`
		PacketsRecv uint64    `json:"packets_recv"`
		Status      uint32    `json:"status"`
		Version     uint64    `json:"version"`
	}

	result := make([]FlowRow, 0, 1000)

	for rows.Next() {
		var fr FlowRow
		if err := rows.ScanStruct(&fr); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("error fetching flows: %v", err), logic.Internal))
			return
		}
		result = append(result, fr)
	}

	logic.ReturnSuccessResponseWithJson(w, r, result, "flows retrieved successfully")
}

func parseIntOrDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
