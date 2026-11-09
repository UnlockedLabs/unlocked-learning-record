package handlers

import "github.com/UnlockedLabs/unlocked-contracts/features"

// routes is every Learning Record route, with the paths and feature they had
// in core (EN-195). Each is open to any logged-in user with the
// learning_record feature.
func (srv *server) routes() []route {
	lr := []features.Access{features.LearningRecord}
	return []route{
		{Pattern: "GET /api/learning-record/facilities", Handler: handle(srv.handleIndexFacilities), Features: lr},
		{Pattern: "GET /api/learning-record/entries", Handler: handle(srv.handleIndexEntries), Features: lr},
		{Pattern: "POST /api/learning-record/entries", Handler: handle(srv.handleCreateEntry), Features: lr},
		{Pattern: "PUT /api/learning-record/entries/{id}", Handler: handle(srv.handleUpdateEntry), Features: lr},
		{Pattern: "DELETE /api/learning-record/entries/{id}", Handler: handle(srv.handleDeleteEntry), Features: lr},
		{Pattern: "GET /api/learning-record/draft", Handler: handle(srv.handleGetDraft), Features: lr},
		{Pattern: "PUT /api/learning-record/draft", Handler: handle(srv.handleUpsertDraft), Features: lr},
		{Pattern: "DELETE /api/learning-record/draft", Handler: handle(srv.handleDeleteDraft), Features: lr},
	}
}
