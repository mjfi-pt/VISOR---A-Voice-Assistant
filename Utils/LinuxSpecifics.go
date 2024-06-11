//go:build linux

/*******************************************************************************
 * Copyright 2023-2024 Edw590
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

package Utils

import (
	"strings"
)

/*
RunningAsAdminPROCESSES checks if the program is running as administrator/root.

-----------------------------------------------------------

– Returns:
  - true if the program is running as admin, false otherwise
*/
func RunningAsAdminPROCESSES() bool {
	stdOutErrCmd, err := ExecCmdSHELL([]string{"id -u"})
	if nil != err {
		return false
	}

	if 0 != stdOutErrCmd.Exit_code {
		return false
	}

	return "0" == strings.TrimSpace(stdOutErrCmd.Stdout_str)
}

/*
HideConsoleWindowPROCESSES hides the console window of the program.

Notice: on Windows only works if the program is started with conhost.exe (always is except when it's started by the
new Windows Terminal). So use StartConAppPROCESSES() to start the program with conhost.exe.
 */
func HideConsoleWindowPROCESSES() {
	// TODO See if it's needed on Linux too and find a way
}
