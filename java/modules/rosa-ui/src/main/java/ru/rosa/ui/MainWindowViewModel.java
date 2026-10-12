package ru.rosa.ui;

import java.util.Objects;
import ru.rosa.application.usecase.GetApplicationInfoQuery;
import ru.rosa.domain.model.ProductIdentity;

final class MainWindowViewModel {
    private final GetApplicationInfoQuery applicationInfo;

    MainWindowViewModel(GetApplicationInfoQuery applicationInfo) {
        this.applicationInfo = Objects.requireNonNull(applicationInfo, "applicationInfo");
    }

    MainWindowState load() {
        ProductIdentity identity = applicationInfo.execute();
        return new MainWindowState(
                identity.previewTitle(),
                "Эталонная версия Go: " + identity.goBaselineVersion(),
                identity.migrationStage().displayName());
    }
}

