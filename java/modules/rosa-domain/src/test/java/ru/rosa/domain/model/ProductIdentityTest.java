package ru.rosa.domain.model;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import org.junit.jupiter.api.Test;

class ProductIdentityTest {
    @Test
    void normalizesIdentityAndBuildsPreviewTitle() {
        ProductIdentity identity = new ProductIdentity(
                "  РОСА  ",
                " 7.3.11.824-RC ",
                new MigrationStage(1, "Архитектурный каркас"));

        assertEquals("РОСА", identity.productName());
        assertEquals("7.3.11.824-RC", identity.goBaselineVersion());
        assertEquals("РОСА · Java Preview", identity.previewTitle());
        assertEquals("Этап 1 · Архитектурный каркас", identity.migrationStage().displayName());
    }

    @Test
    void rejectsIncompleteIdentity() {
        MigrationStage stage = new MigrationStage(1, "Каркас");
        assertThrows(IllegalArgumentException.class, () -> new ProductIdentity(" ", "824-RC", stage));
        assertThrows(IllegalArgumentException.class, () -> new MigrationStage(-1, "Каркас"));
    }
}

