/*******************************************************************************
 * Copyright 2023-2024 The V.I.S.O.R. authors
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 ******************************************************************************/

package SystemChecker

import (
	"Utils"
	"Utils/ModsFileInfo"
	"Utils/UtilsSWA"
	"VISOR_Client/ClientRegKeys"
	"github.com/distatus/battery"
	"github.com/go-vgo/robotgo"
	"github.com/itchyny/volume-go"
	"github.com/schollz/wifiscan"
	"github.com/yusufpapurcu/wmi"
	"runtime"
	"strings"
	"time"
)

const SCAN_WIFI_EACH_S int64 = 60
var last_check_wifi_when_s int64 = 0

var device_info_GL *ModsFileInfo.DeviceInfo

type _Battery struct {
	power_connected bool
	level           int
}

type _MousePosition struct {
	x int
	y int
}

// https://learn.microsoft.com/en-us/windows/win32/wmicoreprov/wmimonitorbrightness
type WmiMonitorBrightness struct {
	CurrentBrightness uint8
}

var (
	realMain      Utils.RealMain = nil
	moduleInfo_GL Utils.ModuleInfo
	modGenInfo_GL *ModsFileInfo.Mod10GenInfo
)
func Start(module *Utils.Module) {Utils.ModStartup(realMain, module)}
func init() {realMain =
	func(module_stop *bool, moduleInfo_any any) {
		moduleInfo_GL = moduleInfo_any.(Utils.ModuleInfo)
		modGenInfo_GL = &Utils.Gen_settings_GL.MOD_10

		var curr_mouse_position _MousePosition

		device_info_GL = &modGenInfo_GL.Device_info

		var wifi_on bool
		var wifi_networks []ModsFileInfo.ExtBeacon
		for {
			if time.Now().Unix() >= last_check_wifi_when_s + SCAN_WIFI_EACH_S {
				// Every 3 minutes, update the wifi networks
				wifi_on, wifi_networks = getWifiNetworks()

				last_check_wifi_when_s = time.Now().Unix()
			}

			// Connectivity information
			device_info_GL.System_state.Connectivity_info = ModsFileInfo.ConnectivityInfo{
				Airplane_mode_enabled: false,
				Wifi_enabled:          wifi_on,
				Bluetooth_enabled:     false,
				Mobile_data_enabled:   false,
				Wifi_networks:         wifi_networks,
				Bluetooth_devices:     nil,
			}

			// Battery information
			var battery_level int = getBatteryInfo().level
			var power_connected bool = getBatteryInfo().power_connected
			UtilsSWA.GetValueREGISTRY(ClientRegKeys.K_BATTERY_LEVEL).SetInt(int32(battery_level), false)
			UtilsSWA.GetValueREGISTRY(ClientRegKeys.K_POWER_CONNECTED).SetBool(power_connected, false)

			device_info_GL.System_state.Battery_info = ModsFileInfo.BatteryInfo{
				Level:           battery_level,
				Power_connected: power_connected,
			}

			// Monitor information
			var screen_brightness int = getBrightness(device_info_GL.System_state.Monitor_info.Brightness)
			UtilsSWA.GetValueREGISTRY(ClientRegKeys.K_SCREEN_BRIGHTNESS).SetInt(int32(screen_brightness), false)

			device_info_GL.System_state.Monitor_info = ModsFileInfo.MonitorInfo{
				Screen_on:  true,
				Brightness: screen_brightness,
			}

			// Sound information
			var sound_volume int = getSoundVolume(device_info_GL.System_state.Sound_info.Volume)
			var sound_muted bool = getSoundMuted(device_info_GL.System_state.Sound_info.Muted)
			UtilsSWA.GetValueREGISTRY(ClientRegKeys.K_SOUND_VOLUME).SetInt(int32(sound_volume), false)
			UtilsSWA.GetValueREGISTRY(ClientRegKeys.K_SOUND_MUTED).SetBool(sound_muted, false)

			device_info_GL.System_state.Sound_info = ModsFileInfo.SoundInfo{
				Volume: sound_volume,
				Muted:  sound_muted,
			}

			// Check if the device is being used by checking if the mouse is moving
			var x, y int = robotgo.Location()
			if x != curr_mouse_position.x || y != curr_mouse_position.y {
				curr_mouse_position.x = x
				curr_mouse_position.y = y

				device_info_GL.Last_time_used_s = time.Now().Unix()
			}

			if Utils.WaitWithStopTIMEDATE(module_stop, 1) {
				return
			}
		}
	}
}

func GetDeviceInfoText() string {
	return *Utils.ToJsonGENERAL(device_info_GL)
}

func getBatteryInfo() _Battery {
	batteries, err := battery.GetAll()
	if err != nil || len(batteries) == 0 {
		return _Battery{}
	}

	var b *battery.Battery = batteries[0]

	return _Battery{
		power_connected: b.State.Raw != battery.Discharging,
		level:           int(b.Current / b.Full * 100),
	}
}

func getBrightness(prev int) int {
	var dst []WmiMonitorBrightness
	err := wmi.QueryNamespace("SELECT CurrentBrightness FROM WmiMonitorBrightness", &dst, "root/wmi")
	if err != nil {
		return prev
	}

	if len(dst) > 0 {
		return int(dst[0].CurrentBrightness)
	}

	return prev
}

func getSoundVolume(prev int) int {
	vol, err := volume.GetVolume()
	if err != nil {
		return prev
	}

	return vol
}

func getSoundMuted(prev bool) bool {
	muted, err := volume.GetMuted()
	if err != nil {
		return prev
	}

	return muted
}

func getWifiNetworks() (bool, []ModsFileInfo.ExtBeacon) {
	if runtime.GOOS == "windows" {
		// Request a Wi-Fi scan first (wifiscan.Scan() doesn't do it on Windows)
		_, _ = Utils.ExecCmdSHELL([]string{".\\external\\WlanScan.exe"})
	}

	// Then get the cached results on Windows or request a scan and get results
	// on Linux.
	var num_tries int = 1
	if runtime.GOOS == "windows" {
		// I don't know how much time after the scan the results are ready, so
		// 10 seconds seems like a good number.
		// If it's on Linux, shouldn't have problem I think.
		num_tries = 10
	}
	for i := 0; i < num_tries; i++ {
		wifi_nets, err := wifiscan.Scan()
		if err != nil {
			return false, nil
		}

		var wifi_networks []ModsFileInfo.ExtBeacon = nil
		for _, wifi_net := range wifi_nets {
			wifi_networks = append(wifi_networks, ModsFileInfo.ExtBeacon{
				Name:    wifi_net.SSID,
				Address: strings.ToUpper(wifi_net.BSSID),
				RSSI:    wifi_net.RSSI,
			})
		}

		if len(wifi_networks) != 0 {
			return true, wifi_networks
		}

		time.Sleep(1 * time.Second)
	}

	return true, nil
}
