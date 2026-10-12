package ru.rosa.domain.model;

import java.util.Objects;

/** Stable product identity shared by the legacy and migrated applications. */
public record ProductIdentity(
        String productName,
        String goBaselineVersion,
        MigrationStage migrationStage) {

    public ProductIdentity {
        productName = requireText(productName, "Product name");
        goBaselineVersion = requireText(goBaselineVersion, "Go baseline version");
        migrationStage = Objects.requireNonNull(migrationStage, "Migration stage must not be null");
    }

    public String previewTitle() {
        return productName + " · Java Preview";
    }

    private static String requireText(String value, String field) {
        Objects.requireNonNull(value, field + " must not be null");
        String normalized = value.strip();
        if (normalized.isEmpty()) {
            throw new IllegalArgumentException(field + " must not be blank");
        }
        return normalized;
    }
}

