package ru.rosa.application.usecase;

import static org.junit.jupiter.api.Assertions.assertSame;

import org.junit.jupiter.api.Test;
import ru.rosa.domain.model.MigrationStage;
import ru.rosa.domain.model.ProductIdentity;

class GetApplicationInfoServiceTest {
    @Test
    void delegatesToConfiguredPort() {
        ProductIdentity expected = new ProductIdentity("РОСА", "7.3.11.824-RC", new MigrationStage(1, "Каркас"));
        GetApplicationInfoService service = new GetApplicationInfoService(() -> expected);

        assertSame(expected, service.execute());
    }
}

