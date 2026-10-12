package ru.rosa.ui;

import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.Parent;
import javafx.scene.control.Label;
import javafx.scene.layout.VBox;
import ru.rosa.application.usecase.GetApplicationInfoQuery;

public final class MainWindow {
    private final MainWindowState state;

    public MainWindow(GetApplicationInfoQuery applicationInfo) {
        this.state = new MainWindowViewModel(applicationInfo).load();
    }

    public String title() {
        return state.title();
    }

    public Parent createContent() {
        Label title = new Label(state.title());
        title.getStyleClass().add("title");

        Label baseline = new Label(state.baseline());
        baseline.getStyleClass().add("baseline");

        Label stage = new Label(state.stage());
        stage.getStyleClass().add("stage");

        VBox content = new VBox(16, title, baseline, stage);
        content.setAlignment(Pos.CENTER);
        content.setPadding(new Insets(36));
        content.getStyleClass().add("root-pane");
        return content;
    }
}

