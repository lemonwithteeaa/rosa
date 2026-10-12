package ru.rosa.launcher;

import javafx.application.Application;
import javafx.scene.Scene;
import javafx.stage.Stage;
import ru.rosa.application.usecase.GetApplicationInfoQuery;
import ru.rosa.application.usecase.GetApplicationInfoService;
import ru.rosa.compatibility.ClasspathGoBaselineAdapter;
import ru.rosa.ui.MainWindow;

/** Application composition root. */
public final class RosaLauncher extends Application {
    public static void main(String[] args) {
        launch(args);
    }

    @Override
    public void start(Stage stage) {
        GetApplicationInfoQuery applicationInfo =
                new GetApplicationInfoService(new ClasspathGoBaselineAdapter());
        MainWindow mainWindow = new MainWindow(applicationInfo);
        Scene scene = new Scene(mainWindow.createContent(), 760, 460);
        scene.getStylesheets().add(
                RosaLauncher.class.getResource("/ru/rosa/ui/rosa.css").toExternalForm());

        stage.setTitle(mainWindow.title());
        stage.setMinWidth(640);
        stage.setMinHeight(400);
        stage.setScene(scene);
        stage.show();
    }
}

