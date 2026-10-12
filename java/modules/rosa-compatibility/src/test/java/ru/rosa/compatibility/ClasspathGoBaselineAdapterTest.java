package ru.rosa.compatibility;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;
import ru.rosa.domain.model.ProductIdentity;

class ClasspathGoBaselineAdapterTest {
    @Test
    void loadsPinnedGoBaseline() {
        ProductIdentity identity = new ClasspathGoBaselineAdapter().load();

        assertEquals("РОСА", identity.productName());
        assertEquals("7.3.11.824-RC", identity.goBaselineVersion());
        assertEquals(1, identity.migrationStage().number());
    }
}

