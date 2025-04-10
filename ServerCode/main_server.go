/*******************************************************************************
 * Copyright 2023-2025 The V.I.S.O.R. authors
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

package main

import (
	"ModulesManager"
	"Utils"
	"VISOR_Server/ServerRegKeys"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

var modDirsInfo_GL   Utils.ModDirsInfo
func main() {
	var module Utils.Module = Utils.Module{
		Num:     Utils.NUM_MOD_VISOR,
		Name:    Utils.GetModNameMODULES(Utils.NUM_MOD_VISOR),
		Stop:    false,
		Stopped: false,
		Enabled: true,
	}
	Utils.ModStartup2(realMain, &module, true)
}
func realMain(module_stop *bool, moduleInfo_any any) {
	modDirsInfo_GL = moduleInfo_any.(Utils.ModDirsInfo)

	if Utils.GetGenSettings().Device_settings.Id == "" || Utils.GetGenSettings().Device_settings.Type_ == "" ||
			Utils.GetGenSettings().Device_settings.Description == "" {
		log.Println("Device settings incomplete. Please enter the missing one(s):")
		if Utils.GetGenSettings().Device_settings.Id == "" {
			Utils.GetGenSettings().Device_settings.Id = Utils.GetInputString("Unique device ID: ")
		}
		if Utils.GetGenSettings().Device_settings.Type_ == "" {
			Utils.GetGenSettings().Device_settings.Type_ = Utils.GetInputString("Device type (for example " +
				"\"computer\"): ")
		}
		if Utils.GetGenSettings().Device_settings.Description == "" {
			Utils.GetGenSettings().Device_settings.Description = Utils.GetInputString("Device description (for " +
				"example the model, \"Legion Y520\"): ")
		}
	}

	if err := Utils.ReadSettingsFile(true); err != nil {
		log.Println("Failed to load user settings. Exiting...")

		return
	}

	if !Utils.RunningAsAdminPROCESSES() {
		log.Println("Not running as administrator/root. Exiting...")

		return
	}

	ServerRegKeys.RegisterValues()

	var modules []Utils.Module
	for i := 0; i < Utils.MODS_ARRAY_SIZE; i++ {
		modules = append(modules, Utils.Module{
			Num:     i,
			Name:    Utils.GetModNameMODULES(i),
			Stop:    true,
			Stopped: true,
			Enabled: true,
		})
	}
	modules[Utils.NUM_MOD_VISOR].Stop = false
	modules[Utils.NUM_MOD_VISOR].Stopped = false
	// The Manager needs to be started first. It'll handle the others.
	modules[Utils.NUM_MOD_ModManager].Stop = false

	ModulesManager.Start(modules)

	handleCtrlCGracefully(module_stop)

	var status bool = Utils.WasArgUsedGENERAL(os.Args, "--status")

	for {
		// Wait forever while the other modules do their work
		if status {
			printModulesStatus(modules)
		}

		Utils.WriteSettingsFile(true)

		if Utils.WaitWithStopDATETIME(module_stop, 5) {
			break
		}
	}

	Utils.CloseCommsChannels()
	Utils.SignalModulesStopMODULES(modules)
}

func handleCtrlCGracefully(module_stop *bool) {
	// Copied from https://gist.github.com/jnovack/297cee036f3e5a430aa9444c0ae1b06d
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<- c

		*module_stop = true
	}()
}

func printModulesStatus(modules []Utils.Module) {
	log.Println("--------------------------------")
	for _, module := range modules {
		log.Println("--- " + module.Name + " ---")
		log.Println("- Enabled: " + strconv.FormatBool(module.Enabled))
		log.Println("- To stop: " + strconv.FormatBool(module.Stop))
		log.Println("- Support: " + strconv.FormatBool(Utils.IsModSupportedMODULES(module.Num)))
		log.Println("- Running: " + strconv.FormatBool(!module.Stopped))
	}
}
