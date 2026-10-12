package ru.rosa.ui;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;
import ru.rosa.domain.model.MigrationStage;
import ru.rosa.domain.model.ProductIdentity;

class MainWindowViewModelTest {
    @Test
    void mapsDomainIdentityWithoutJavaFxRuntime() {
        ProductIdentity identity = new ProductIdentity(
                "РОСА", "7.3.11.824-RC", new MigrationStage(1, "Архитектурный каркас"));
        MainWindowViewModel viewModel = new MainWindowViewModel(() -> identity);

        MainWindowState state = viewModel.load();

        assertEquals("РОСА · Java Preview", state.title());
        assertEquals("Эталонная версия Go: 7.3.11.824-RC", state.baseline());
        assertEquals("Этап 1 · Архитектурный каркас", state.stage());
    }
}

