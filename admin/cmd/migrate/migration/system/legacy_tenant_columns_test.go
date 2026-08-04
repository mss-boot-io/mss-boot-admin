package system

import "testing"

func TestLegacyTenantColumnsQuery(t *testing.T) {
	if _, ok := legacyTenantColumnsQuery("postgres"); !ok {
		t.Fatal("postgres should be supported")
	}
	if _, ok := legacyTenantColumnsQuery("mysql"); !ok {
		t.Fatal("mysql should be supported")
	}
	if _, ok := legacyTenantColumnsQuery("sqlite"); ok {
		t.Fatal("sqlite should be ignored")
	}
}

func TestRelaxLegacyTenantColumnSQL(t *testing.T) {
	sql, ok := relaxLegacyTenantColumnSQL("postgres", legacyTenantColumn{TableName: `mss_boot"menus`})
	if !ok {
		t.Fatal("postgres should build SQL")
	}
	want := `ALTER TABLE "mss_boot""menus" ALTER COLUMN "tenant_id" DROP NOT NULL`
	if sql != want {
		t.Fatalf("postgres SQL mismatch\nwant: %s\n got: %s", want, sql)
	}

	sql, ok = relaxLegacyTenantColumnSQL("mysql", legacyTenantColumn{
		TableName:  "mss_boot_menus",
		ColumnType: "varchar(36)",
	})
	if !ok {
		t.Fatal("mysql should build SQL")
	}
	want = "ALTER TABLE `mss_boot_menus` MODIFY COLUMN `tenant_id` varchar(36) NULL"
	if sql != want {
		t.Fatalf("mysql SQL mismatch\nwant: %s\n got: %s", want, sql)
	}
}
