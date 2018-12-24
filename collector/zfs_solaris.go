// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !nozfs

package collector

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("zfs", defaultEnabled, NewZFSCollector)
}

type zfsCollector struct{}

func NewZFSCollector() (Collector, error) {
	return &zfsCollector{}, nil
}

func (c *zfsCollector) Update(ch chan<- prometheus.Metric) error {
	bs, err := exec.Command("kstat", "-j", "/zfs|zone_zfs/:::").Output()
	if err != nil {
		return fmt.Errorf("executing kstat: %v", err)
	}

	var stats []*kstatEntry
	if err := json.Unmarshal(bs, &stats); err != nil {
		return fmt.Errorf("parsing kstat output: %v", err)
	}

	for _, stat := range stats {
		switch stat.Class {
		case "misc":
			if err := c.updateMisc(ch, stat); err != nil {
				return err
			}
		case "disk":
			if err := c.updatePool(ch, stat); err != nil {
				return err
			}
		case "zone_zfs":
			if err := c.updateZoneStats(ch, stat); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *zfsCollector) updateMisc(ch chan<- prometheus.Metric, stat *kstatEntry) error {
	for k, v := range stat.Data {
		val, ok := v.(float64)
		if !ok {
			return fmt.Errorf("couldn't decode stat value %q as float64", v)
		}
		metricName := fmt.Sprintf("%s_%s", stat.Name, k)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "zfs", metricName),
				metricName,
				nil,
				nil,
			),
			prometheus.UntypedValue,
			val,
		)
	}
	return nil
}

func (c *zfsCollector) updatePool(ch chan<- prometheus.Metric, stat *kstatEntry) error {
	for k, v := range stat.Data {
		val, ok := v.(float64)
		if !ok {
			return fmt.Errorf("couldn't decode stat value %q as float64", v)
		}
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "zfs_zpool", k),
				k,
				[]string{"zpool"},
				nil,
			),
			prometheus.UntypedValue,
			val,
			stat.Name,
		)
	}
	return nil
}

func (c *zfsCollector) updateZoneStats(ch chan<- prometheus.Metric, stat *kstatEntry) error {
	zonename, ok := stat.Data["zonename"].(string)
	if !ok {
		return fmt.Errorf("no zonename in zone_zfs stat %q", stat.Name)
	}

	for k, v := range stat.Data {
		if k == "zonename" {
			continue
		}
		val, ok := v.(float64)
		if !ok {
			return fmt.Errorf("couldn't decode stat value %q as float64", v)
		}
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "zfs_zone", k),
				k,
				[]string{"zone"},
				nil,
			),
			prometheus.UntypedValue,
			val,
			zonename,
		)
	}

	return nil
}

type kstatEntry struct {
	Module string                 `json:"module"`
	Name   string                 `json:"name"`
	Class  string                 `json:"class"`
	Data   map[string]interface{} `json:"data"`
}
