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
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	proLogic "github.com/gravitl/netmaker/pro/logic"
)

func FlowHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/flows", logic.SecurityCheck(true, http.HandlerFunc(handleListFlows))).Methods(http.MethodGet)
}

const (
	querySelect = `
SELECT
	flow_id, host_id, host_name, network_id,
	protocol, src_port, dst_port,
	icmp_type, icmp_code, direction,
	src_ip, src_type, src_entity_id, src_entity_name,
	dst_ip, dst_type, dst_entity_id, dst_entity_name,
	start_ts, end_ts,
	bytes_sent, bytes_recv,
	packets_sent, packets_recv,
	status, version
FROM flows`
	queryOrder = `
ORDER BY version DESC
LIMIT ? OFFSET ?`
)

// FlowRow represents a single flow log entry
type FlowRow struct {
	FlowID        string    `ch:"flow_id" json:"flow_id"`
	HostID        string    `ch:"host_id" json:"host_id"`
	HostName      string    `ch:"host_name" json:"host_name"`
	NetworkID     string    `ch:"network_id" json:"network_id"`
	Protocol      uint16    `ch:"protocol" json:"protocol"`
	SrcPort       uint16    `ch:"src_port" json:"src_port"`
	DstPort       uint16    `ch:"dst_port" json:"dst_port"`
	ICMPType      uint8     `ch:"icmp_type" json:"icmp_type"`
	ICMPCode      uint8     `ch:"icmp_code" json:"icmp_code"`
	Direction     string    `ch:"direction" json:"direction"`
	SrcIP         string    `ch:"src_ip" json:"src_ip"`
	SrcType       string    `ch:"src_type" json:"src_type"`
	SrcEntityID   string    `ch:"src_entity_id" json:"src_entity_id"`
	SrcEntityName string    `ch:"src_entity_name" json:"src_entity_name"`
	DstIP         string    `ch:"dst_ip" json:"dst_ip"`
	DstType       string    `ch:"dst_type" json:"dst_type"`
	DstEntityID   string    `ch:"dst_entity_id" json:"dst_entity_id"`
	DstEntityName string    `ch:"dst_entity_name" json:"dst_entity_name"`
	StartTs       time.Time `ch:"start_ts" json:"start_ts"`
	EndTs         time.Time `ch:"end_ts" json:"end_ts"`
	BytesSent     uint64    `ch:"bytes_sent" json:"bytes_sent"`
	BytesRecv     uint64    `ch:"bytes_recv" json:"bytes_recv"`
	PacketsSent   uint64    `ch:"packets_sent" json:"packets_sent"`
	PacketsRecv   uint64    `ch:"packets_recv" json:"packets_recv"`
	Status        uint32    `ch:"status" json:"status"`
	Version       time.Time `ch:"version" json:"version"`
}

// @Summary     List flow logs
// @Router      /api/v1/flows [get]
// @Tags        Traffic Logs
// @Security    oauth
// @Produce     json
// @Param       network_id query string false "Filter by network ID"
// @Param       from query string false "Start time in RFC3339 format"
// @Param       to query string false "End time in RFC3339 format"
// @Param       src_type query string false "Source type filter"
// @Param       src_entity_id query string false "Source entity ID filter"
// @Param       dst_type query string false "Destination type filter"
// @Param       dst_entity_id query string false "Destination entity ID filter"
// @Param       protocol query string false "Protocol filter"
// @Param       node_id query string false "Node ID filter"
// @Param       username query string false "Username filter"
// @Param       page query int false "Page number"
// @Param       per_page query int false "Items per page (max 1000)"
// @Success     200 {array} FlowRow
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func handleListFlows(w http.ResponseWriter, r *http.Request) {
	if !proLogic.GetFeatureFlags().EnableFlowLogs || !logic.GetServerSettings().EnableFlowLogs {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("flow logs not enabled"), logic.Forbidden))
		return
	}

	q := r.URL.Query()

	// TODO: handle query filters better
	var (
		whereParts []string
		args       []any
	)

	// 0. Network filter.
	networkID := q.Get("network_id")
	if networkID != "" {
		whereParts = append(whereParts, "network_id = ?")
		args = append(args, networkID)
	}

	// 1. Time filtering (version: UInt64 timestamp in ms)
	fromStr := q.Get("from")
	toStr := q.Get("to")

	if fromStr != "" {
		fromVal, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid 'from' timestamp: %v", err), logic.BadReq))
			return
		}
		whereParts = append(whereParts, "version >= ?")
		args = append(args, fromVal)
	}

	if toStr != "" {
		toVal, err := time.Parse(time.RFC3339, toStr)
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

	// 5. Node filter
	nodeID := q.Get("node_id")
	if nodeID != "" {
		node, err := logic.GetNodeByID(nodeID)
		if err != nil {
			errType := logic.Internal
			if database.IsEmptyRecord(err) {
				errType = logic.BadReq
			}
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("error fetching node with id %s: %v", nodeID, err), errType))
			return
		}

		if networkID == "" {
			whereParts = append(whereParts, "network_id = ?")
			args = append(args, node.Network)
		} else {
			if networkID != node.Network {
				logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("node with id %s does not belong to network %s", nodeID, networkID), logic.BadReq))
				return
			}
		}

		whereParts = append(whereParts, "host_id = ?")
		args = append(args, node.HostID)
	}

	// 6. User filter
	username := q.Get("username")
	if username != "" {
		if srcTypeStr != "" || dstTypeStr != "" ||
			srcEntity != "" || dstEntity != "" {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot provide username filter along with src/dst type and id filters"), logic.BadReq))
			return
		}

		srcTypeStr = "user"
		srcEntity = username
		dstTypeStr = "user"
		dstEntity = username

		whereParts = append(whereParts, "((src_type = ? AND src_entity_id = ?) OR (dst_type = ? AND dst_entity_id = ?))")
		args = append(args, srcTypeStr, srcEntity, dstTypeStr, dstEntity)
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

	conn, err := ch.FromContext(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r,
			logic.FormatError(fmt.Errorf("clickhouse connection not available: %v", err), logic.Internal))
		return
	}
	rows, err := conn.Query(r.Context(), query, args...)
	if err != nil {
		logic.ReturnErrorResponse(w, r,
			logic.FormatError(fmt.Errorf("error fetching flows: %v", err), logic.Internal))
		return
	}
	defer rows.Close()

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
