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

package CmdsExecutor

import (
	"GMan/GMan"
	"strconv"
	"time"
)

func getTasksList(tasks_ids []string, cmd_variant string) string {
	var speak string = ""

	for _, task_id := range tasks_ids {
		var task = GMan.GetTask(task_id)
		if task == nil {
			continue
		}

		var add_task bool = false
		task_date, err := time.Parse("2006-01-02", task.Date)
		if err == nil { // Meaning, the date is empty
			switch cmd_variant {
			case RET_31_TODAY:
				if task_date.Day() == time.Now().Day() {
					add_task = true
				}
			case RET_31_TOMORROW:
				if task_date.Day() == time.Now().AddDate(0, 0, 1).Day() {
					add_task = true
				}
			}
		}
		if add_task || task.Date == "" {
			speak += task.Title + "; "
		}
	}

	var when string
	if cmd_variant == RET_31_TODAY {
		when = "today"
	} else if cmd_variant == RET_31_TOMORROW {
		when = "tomorrow"
	}
	if speak == "" {
		speak = "You have no tasks found for " + when + "."
	} else {
		speak = "Your list of tasks for " + when + ": " + speak + "."
	}

	return speak
}

func getEventsList(events_ids []string, cmd_variant string) string {
	var speak string = ""

	for _, event_id := range events_ids {
		var event = GMan.GetEvent(event_id)
		if event == nil {
			continue
		}

		event_date_time, _ := time.Parse(time.RFC3339, event.Start_time)

		var add_event bool = false
		switch cmd_variant {
			case RET_31_TODAY:
				if event_date_time.Day() == time.Now().Day() {
					add_event = true
				}
			case RET_31_TOMORROW:
				if event_date_time.Day() == time.Now().AddDate(0, 0, 1).Day() {
					add_event = true
				}
			case RET_31_THIS_WEEK:
				if event_date_time.Weekday() >= time.Now().Weekday() {
					add_event = true
				}
			case RET_31_NEXT_WEEK:
				var days_until_next_monday int = int((8 - time.Now().Weekday()) % 7)
				if days_until_next_monday == 0 {
					days_until_next_monday = 7
				}
				next_monday := time.Now().AddDate(0, 0, days_until_next_monday)
				if event_date_time.Day() >= next_monday.Day() &&
					event_date_time.Day() < next_monday.AddDate(0, 0, 7).Day() {
					add_event = true
				}
		}
		if add_event {
			var event_on string = ""
			if cmd_variant == RET_31_THIS_WEEK || cmd_variant == RET_31_NEXT_WEEK {
				event_on = " on " + event_date_time.Weekday().String()
			}
			speak += event.Summary + event_on + " at " + event_date_time.Format("15:04") +
				" for " + getEventDuration(event.Duration_min) + "; "
		}
	}

	var when string
	if cmd_variant == RET_31_TODAY {
		when = "today"
	} else if cmd_variant == RET_31_TOMORROW {
		when = "tomorrow"
	} else if cmd_variant == RET_31_THIS_WEEK {
		when = "this week"
	} else if cmd_variant == RET_31_NEXT_WEEK {
		when = "next week"
	}
	if speak == "" {
		speak = "You have no events found for " + when + "."
	} else {
		speak = "Your list of events for " + when + ": " + speak + "."
	}

	return speak
}

func getEventDuration(min int64) string {
	// Return in hours if more than 60 minutes, or days if more than 24 hours, etc. Else, return in minutes.
	if min >= 60 {
		if min >= 24*60 {
			if min >= 7*24*60 {
				return strconv.Itoa(int(min/(7*24*60))) + " weeks"
			}
			return strconv.Itoa(int(min/(24*60))) + " days"
		}
		return strconv.Itoa(int(min/60)) + " hours"
	}

	return strconv.Itoa(int(min)) + " minutes"
}
