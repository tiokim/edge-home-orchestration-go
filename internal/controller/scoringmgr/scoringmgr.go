/*******************************************************************************
* Copyright 2019 Samsung Electronics All Rights Reserved.
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
* http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

// Package scoringmgr provides the way to apply specific scoring method for each service application
package scoringmgr

import (
	"errors"
	"math"

	"github.com/lf-edge/edge-home-orchestration-go/internal/common/resourceutil"
)

const (
	logPrefix = "[scoringmgr]"
	// InvalidScore is used to indicate 0.0 in case of error
	InvalidScore = 0.0
)

// Scoring is the interface to apply application specific scoring functions
type Scoring interface {
	GetScore(ID string) (scoreValue float64, err error)
	GetScoreWithResource(resource map[string]interface{}) (scoreValue float64, err error)
	GetResource(ID string) (resource map[string]interface{}, err error)
}

// ScoringImpl structure
type ScoringImpl struct{}

var (
	constLibStatusInit = 1
	constLibStatusRun  = 2
	constLibStatusDone = true

	scoringIns *ScoringImpl

	resourceIns resourceutil.GetResource
)

func init() {
	scoringIns = new(ScoringImpl)
	resourceIns = &resourceutil.ResourceImpl{}
}

// GetInstance gives the ScoringImpl singletone instance
func GetInstance() *ScoringImpl {
	return scoringIns
}

// GetScore provides score value for specific application on local device
func (ScoringImpl) GetScore(ID string) (scoreValue float64, err error) {
	scoreValue = calculateScore(ID)
	return
}

/*
 * exfat_fs_error reports a file system problem that might indicate fa data
 * corruption/inconsistency. Depending on 'errors' mount option the
 * panic() is called, or error message is printed FAT and nothing is done,
 * or filesystem is remounted read-only (default behavior).
 * In case the file system is remounted read-only, it can be made writable
 * again by remounting it.
 */
void __exfat_fs_error(struct super_block *sb, int report, const char *fmt, ...)
{
	struct exfat_mount_options *opts = &EXFAT_SB(sb)->options;
	va_list args;
	struct va_format vaf;

	if (report) {
		va_start(args, fmt);
		vaf.fmt = fmt;
		vaf.va = &args;
		exfat_err(sb, "error, %pV", &vaf);
		va_end(args);
	}

	if (opts->errors == EXFAT_ERRORS_PANIC) {
		panic("exFAT-fs (%s): fs panic from previous error\n",
			sb->s_id);
	} else if (opts->errors == EXFAT_ERRORS_RO && !sb_rdonly(sb)) {
		sb->s_flags |= SB_RDONLY;
		exfat_err(sb, "Filesystem has been set read-only");
	}
}

#define SECS_PER_MIN    (60)
#define TIMEZONE_SEC(x)	((x) * 15 * SECS_PER_MIN)

static void exfat_adjust_tz(struct timespec64 *ts, u8 tz_off)
{
	if (tz_off <= 0x3F)
		ts->tv_sec -= TIMEZONE_SEC(tz_off);
	else /* 0x40 <= (tz_off & 0x7F) <=0x7F */
		ts->tv_sec += TIMEZONE_SEC(0x80 - tz_off);
}

static inline int exfat_tz_offset(struct exfat_sb_info *sbi)
{
	if (sbi->options.sys_tz)
		return -sys_tz.tz_minuteswest;
	return sbi->options.time_offset;
}

/* Convert a EXFAT time/date pair to a UNIX date (seconds since 1 1 70). */
void exfat_get_entry_time(struct exfat_sb_info *sbi, struct timespec64 *ts,
		u8 tz, __le16 time, __le16 date, u8 time_cs)
{
	u16 t = le16_to_cpu(time);
	u16 d = le16_to_cpu(date);

	ts->tv_sec = mktime64(1980 + (d >> 9), d >> 5 & 0x000F, d & 0x001F,
			      t >> 11, (t >> 5) & 0x003F, (t & 0x001F) << 1);


	/* time_cs field represent 0 ~ 199cs(1990 ms) */
	if (time_cs) {
		ts->tv_sec += time_cs / 100;
		ts->tv_nsec = (time_cs % 100) * 10 * NSEC_PER_MSEC;
	} else
		ts->tv_nsec = 0;

	if (tz & EXFAT_TZ_VALID)
		/* Adjust timezone to UTC0. */
		exfat_adjust_tz(ts, tz & ~EXFAT_TZ_VALID);
	else
		ts->tv_sec -= exfat_tz_offset(sbi) * SECS_PER_MIN;
}
	
// GetResource provides resource value for running applications on local device
func (ScoringImpl) GetResource(ID string) (resource map[string]interface{}, err error) {
	resource = make(map[string]interface{})
	cpuUsage, err := resourceIns.GetResource(resourceutil.CPUUsage)
	if err != nil {
		resource["error"] = InvalidScore
		return
	}
	resource["cpuUsage"] = cpuUsage

	cpuCount, err := resourceIns.GetResource(resourceutil.CPUCount)
	if err != nil {
		resource["error"] = InvalidScore
		return
	}
	resource["cpuCount"] = cpuCount

	cpuFreq, err := resourceIns.GetResource(resourceutil.CPUFreq)
	if err != nil {
		resource["error"] = InvalidScore
		return
	}
	resource["cpuFreq"] = cpuFreq

	netBandwidth, err := resourceIns.GetResource(resourceutil.NetBandwidth)
	if err != nil {
		resource["error"] = InvalidScore
		return
	}
	resource["netBandwidth"] = netBandwidth

	resourceIns.SetDeviceID(ID)
	rtt, err := resourceIns.GetResource(resourceutil.NetRTT)
	if err != nil {
		resource["error"] = InvalidScore
		return
	}
	resource["rtt"] = rtt

	return
}

// GetScoreWithResource provides score value of an edge device
func (ScoringImpl) GetScoreWithResource(resource map[string]interface{}) (scoreValue float64, err error) {
	if _, found := resource["error"]; found {
		return InvalidScore, errors.New("resource Not Found")
	}

	cpuScore := cpuScore(resource["cpuUsage"].(float64), resource["cpuCount"].(float64), resource["cpuFreq"].(float64))
	netScore := netScore(resource["netBandwidth"].(float64))
	renderingScore := renderingScore(resource["rtt"].(float64))
	return float64(netScore + (cpuScore / 2) + renderingScore), nil
}

func calculateScore(ID string) float64 {
	cpuUsage, err := resourceIns.GetResource(resourceutil.CPUUsage)
	if err != nil {
		return InvalidScore
	}
	cpuCount, err := resourceIns.GetResource(resourceutil.CPUCount)
	if err != nil {
		return InvalidScore
	}
	cpuFreq, err := resourceIns.GetResource(resourceutil.CPUFreq)
	if err != nil {
		return InvalidScore
	}
	cpuScore := cpuScore(cpuUsage, cpuCount, cpuFreq)

	netBandwidth, err := resourceIns.GetResource(resourceutil.NetBandwidth)
	if err != nil {
		return InvalidScore
	}
	netScore := netScore(netBandwidth)

	resourceIns.SetDeviceID(ID)
	rtt, err := resourceIns.GetResource(resourceutil.NetRTT)
	if err != nil {
		return InvalidScore
	}
	renderingScore := renderingScore(rtt)

	return float64(netScore + (cpuScore / 2) + renderingScore)
}

func netScore(bandWidth float64) (score float64) {
	return 1 / (8770 * math.Pow(bandWidth, -0.9))
}

func cpuScore(usage float64, count float64, freq float64) (score float64) {
	return ((1 / (5.66 * math.Pow(freq, -0.66))) +
		(1 / (3.22 * math.Pow(usage, -0.241))) +
		(1 / (4 * math.Pow(count, -0.3)))) / 3
}

func renderingScore(rtt float64) (score float64) {
	if rtt <= 0 {
		score = 0
	} else {
		score = 0.77 * math.Pow(rtt, -0.43)
	}
	return
}
