package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type selfCase struct {
	Case struct {
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
		Status   string `json:"status"`
	} `json:"case"`
	Permit *struct {
		SerialNumber string `json:"serialNumber"`
	} `json:"permit"`
}

func runSelfcheck(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second}
	base := "http://" + addr
	for i := 0; i < 20; i++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/readyz", nil)
		if res, e := client.Do(req); e == nil {
			_ = res.Body.Close()
			if res.StatusCode == 200 {
				break
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("就绪探针超时")
		case <-time.After(25 * time.Millisecond):
		}
	}
	now := time.Now().UTC()
	create := map[string]any{"stationCode": "SELF-CHECK", "title": "自检变更案", "effectiveFrom": now.Add(time.Hour), "effectiveUntil": now.Add(2 * time.Hour), "profile": map[string]any{"frequencyHz": 100000000, "bandwidthHz": 10000, "powerWatts": 0.001, "antennaGainDb": 0, "azimuthDegrees": 0, "siteLatitude": 31.2, "siteLongitude": 121.5}}
	var v selfCase
	if e := selfRequest(ctx, client, "POST", base+"/api/v1/change-cases", "planner", "自检规划员", "self-create", create, &v); e != nil {
		return e
	}
	target := map[string]any{"expectedRevision": v.Case.Revision, "name": "远端保护业务", "serviceClass": "safety", "frequencyLowHz": 200000000, "frequencyHighHz": 200010000, "minimumSeparationHz": 100000, "fieldStrengthLimitDbuvm": 30, "ruleReference": "SELF-RULE-1"}
	if e := selfRequest(ctx, client, "POST", base+"/api/v1/change-cases/"+v.Case.ID+"/targets", "planner", "自检规划员", "self-target", target, &v); e != nil {
		return e
	}
	if e := selfRequest(ctx, client, "POST", base+"/api/v1/change-cases/"+v.Case.ID+"/submit", "planner", "自检规划员", "self-submit", map[string]any{"expectedRevision": v.Case.Revision}, &v); e != nil {
		return e
	}
	if v.Case.Status != "reviewed" {
		return fmt.Errorf("自检全量核验未通过: %s", v.Case.Status)
	}
	if e := selfRequest(ctx, client, "POST", base+"/api/v1/change-cases/"+v.Case.ID+"/freeze", "planner", "自检规划员", "self-freeze", map[string]any{"expectedRevision": v.Case.Revision}, &v); e != nil {
		return e
	}
	if e := selfRequest(ctx, client, "POST", base+"/api/v1/change-cases/"+v.Case.ID+"/decision", "leader", "自检技术负责人", "self-approve", map[string]any{"expectedRevision": v.Case.Revision, "decision": "approve", "comment": "自检批准"}, &v); e != nil {
		return e
	}
	if v.Permit == nil || v.Permit.SerialNumber == "" {
		return fmt.Errorf("自检未取得许可")
	}
	var verify struct {
		Valid bool `json:"valid"`
	}
	if e := selfRequest(ctx, client, "GET", base+"/api/v1/change-cases/"+v.Case.ID+"/permit/verify", "", "", "", nil, &verify); e != nil {
		return e
	}
	if !verify.Valid {
		return fmt.Errorf("许可或审计摘要验证失败")
	}
	return nil
}
func selfRequest(ctx context.Context, client *http.Client, method, url, role, actor, key string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, url, reader)
	if e != nil {
		return e
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if role != "" {
		req.Header.Set("X-Role", role)
		req.Header.Set("X-Actor", actor)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, e := client.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if e != nil {
		return e
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d: %s", method, url, res.StatusCode, string(raw))
	}
	if out != nil && len(raw) > 0 {
		if e = json.Unmarshal(raw, out); e != nil {
			return e
		}
	}
	return nil
}
