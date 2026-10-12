package ru.rosa.application.usecase;

import java.util.Objects;
import ru.rosa.application.port.GoBaselinePort;
import ru.rosa.domain.model.ProductIdentity;

public final class GetApplicationInfoService implements GetApplicationInfoQuery {
    private final GoBaselinePort baselinePort;

    public GetApplicationInfoService(GoBaselinePort baselinePort) {
        this.baselinePort = Objects.requireNonNull(baselinePort, "baselinePort");
    }

    @Override
    public ProductIdentity execute() {
        return baselinePort.load();
    }
}

