package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	ch "github.com/gravitl/netmaker/clickhouse"
	"github.com/gravitl/netmaker/logic"
	proLogic "github.com/gravitl/netmaker/pro/logic"
)

func FlowHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/flows", logic.SecurityCheck(true, http.HandlerFunc(handleListFlows))).Methods(http.MethodGet)
}

func handleListFlows(w http.ResponseWriter, r *http.Request) {
	if !proLogic.GetFeatureFlags().EnableFlowLogs {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("flow logs not enabled"), logic.Forbidden))
	}

	rows, err := ch.FromContext(r.Context()).Query(r.Context(), `
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
		FROM flows
		ORDER BY start_ts DESC
		LIMIT 1000
	`)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
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
			http.Error(w, fmt.Sprintf("scan error: %v", err), http.StatusInternalServerError)
			return
		}
		result = append(result, fr)
	}

	logic.ReturnSuccessResponseWithJson(w, r, result, "flows retrieved successfully")
}
