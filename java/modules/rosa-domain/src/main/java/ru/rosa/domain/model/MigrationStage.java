package ru.rosa.domain.model;

import java.util.Objects;

/** A named, ordered checkpoint in the controlled migration. */
public record MigrationStage(int number, String name) {
    public MigrationStage {
        if (number < 0) {
            throw new IllegalArgumentException("Migration stage number must be non-negative");
        }
        name = requireText(name, "Migration stage name");
    }

    public String displayName() {
        return "Этап " + number + " · " + name;
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

