#!/usr/bin/env python3
"""Repair one known v0.7 migration only inside a disposable upgrade fixture."""

from __future__ import annotations

import argparse
from pathlib import Path


OLD_MODULE = "github.com/mss-boot-io/mss-boot/pkg/migration"
CURRENT_MODULE = "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
MIGRATION = Path("cmd/migrate/migration/system/20260403225953_enhance_options.go")
ROLE_MIGRATION = Path("cmd/migrate/migration/system/1691847581348_migrate.go")
MENU_MODEL = Path("models/menu.go")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline", required=True, type=Path)
    parser.add_argument("--candidate-admin", required=True, type=Path)
    args = parser.parse_args()

    baseline_path = args.baseline / MIGRATION
    candidate_path = args.candidate_admin / MIGRATION
    historical = baseline_path.read_text(encoding="utf-8")
    repaired = candidate_path.read_text(encoding="utf-8")

    if "ADD COLUMN IF NOT EXISTS" not in historical:
        raise ValueError("v0.7 fixture no longer contains the expected historical options migration")
    if OLD_MODULE not in historical:
        raise ValueError("v0.7 fixture uses an unexpected framework module path")
    if "ensureOptionsIndex" not in repaired or CURRENT_MODULE not in repaired:
        raise ValueError("candidate options migration is not the expected portable repair")

    baseline_path.write_text(
        repaired.replace(CURRENT_MODULE, OLD_MODULE),
        encoding="utf-8",
    )

    role_path = args.baseline / ROLE_MIGRATION
    role_migration = role_path.read_text(encoding="utf-8")
    historical_query = 'err = tx.Model(&models.Role{}).Where("`default` = ?", true).First(adminRole).Error'
    if historical_query not in role_migration or '"gorm.io/gorm/clause"' in role_migration:
        raise ValueError("v0.7 fixture no longer contains the expected default-role query")
    role_migration = role_migration.replace(
        '"gorm.io/gorm"',
        '"gorm.io/gorm"\n\t"gorm.io/gorm/clause"',
    ).replace(
        historical_query,
        "err = tx.Model(&models.Role{}).\n"
        '\t\t\tWhere(clause.Eq{Column: clause.Column{Name: "default"}, Value: true}).\n'
        "\t\t\tFirst(adminRole).Error",
    )
    role_path.write_text(role_migration, encoding="utf-8")

    menu_path = args.baseline / MENU_MODEL
    menu_model = menu_path.read_text(encoding="utf-8")
    menu_query = 'tx.Model(&Role{}).Where("`default` = ?", true).First(&defaultRole).Error'
    if menu_query not in menu_model or '"gorm.io/gorm/clause"' in menu_model:
        raise ValueError("v0.7 fixture no longer contains the expected menu default-role query")
    menu_model = menu_model.replace(
        '"gorm.io/gorm"',
        '"gorm.io/gorm"\n\t"gorm.io/gorm/clause"',
    ).replace(
        menu_query,
        'tx.Model(&Role{}).Where(clause.Eq{Column: clause.Column{Name: "default"}, Value: true}).'
        "First(&defaultRole).Error",
    )
    menu_path.write_text(menu_model, encoding="utf-8")


if __name__ == "__main__":
    main()
