// Copyright 2026 Quest Financial Technologies S.à r.l.-S., Luxembourg
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.


package api

import (
	"net/http"

	"github.com/QuestFinTech/taskschmiede/internal/compatibility"
	"github.com/QuestFinTech/taskschmiede/internal/onboarding"
)

// handleCompatibility returns the LLM compatibility matrix (public, no auth).
func (a *API) handleCompatibility(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.ListCompletedAttempts()
	if err != nil {
		a.logger.Error("Failed to query compatibility data", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error",
			"Failed to query compatibility data")
		return
	}

	v := onboarding.DefaultInterviewVersion()

	matrix := compatibility.Build(rows, compatibility.Thresholds{
		Pass:        v.PassThreshold,
		Distinction: v.DistinctionThreshold,
		MaxScore:    v.MaxTotalScore(),
		Sections:    v.SectionCount(),
	})

	writeData(w, http.StatusOK, matrix)
}
