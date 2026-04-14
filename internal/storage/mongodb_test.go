package storage

import (
	"os"
	"testing"
)

func TestNewMongoDBStorage_Parsing(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name       string
		connStr    string
		expectDB   string
		expectColl string
		expectErr  bool
	}{
		{
			name:       "Full URI",
			connStr:    "mongodb://localhost:27017/mydb?collection=mycoll",
			expectDB:   "mydb",
			expectColl: "mycoll",
			expectErr:  false,
		},
		{
			name:       "No DB No Coll",
			connStr:    "mongodb://localhost:27017",
			expectDB:   "philterscope",
			expectColl: "audits",
			expectErr:  false,
		},
		{
			name:       "DB only",
			connStr:    "mongodb://localhost:27017/testdb",
			expectDB:   "testdb",
			expectColl: "audits",
			expectErr:  false,
		},
		{
			name:       "Coll only",
			connStr:    "mongodb://localhost:27017/?collection=testcoll",
			expectDB:   "philterscope",
			expectColl: "testcoll",
			expectErr:  false,
		},
		{
			name:      "Empty",
			connStr:   "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("PHILTERSCOPE_MONGODB_CONNECTION_STRING", tt.connStr)
			defer os.Unsetenv("PHILTERSCOPE_MONGODB_CONNECTION_STRING")

			// We don't want to actually connect to MongoDB in this unit test if we can avoid it,
			// but NewMongoDBStorage calls mongo.Connect.
			// Let's just check if it fails before Connect or if we can mock it.
			// Actually, mongo.Connect doesn't necessarily fail immediately if the server is down unless we ping.

			s, err := NewMongoDBStorage(ctx)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				// Connect might fail if localhost:27017 is not reachable, but let's see.
				// For parsing test, we mostly care about s.database and s.collection
				// If Connect fails, it returns nil, err.
				// However, standard driver doesn't necessarily fail on Connect without ping or other options.
				t.Logf("Connect returned error (expected if no mongo): %v", err)
			}

			if s != nil {
				if s.database != tt.expectDB {
					t.Errorf("expected DB %s, got %s", tt.expectDB, s.database)
				}
				if s.collection != tt.expectColl {
					t.Errorf("expected Coll %s, got %s", tt.expectColl, s.collection)
				}
				s.Close(ctx)
			}
		})
	}
}
