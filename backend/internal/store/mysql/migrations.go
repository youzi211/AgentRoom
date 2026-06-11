package mysql

// Migration support is now handled by GORM AutoMigrate.
// See store.go Migrate() method.
//
// The SQL files in migrations/ are kept as reference documentation for the
// intended schema. GORM AutoMigrate will create tables and add missing columns
// based on the model structs in models.go.
//
// For production environments that require version-controlled migrations,
// consider using golang-migrate or a similar tool alongside GORM models.
