import SwiftUI
import SitLabShared
import UserNotifications

@main
struct SitLabApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            ComposeView()
                .ignoresSafeArea(.keyboard)
                .onOpenURL { url in
                    MainViewControllerKt.HandleDeepLink(url: url.absoluteString)
                }
        }
        .onChange(of: scenePhase) { phase in
            if phase == .active {
                MainViewControllerKt.ApplicationDidBecomeActive()
            }
        }
    }
}

final class AppDelegate: NSObject, UIApplicationDelegate, UNUserNotificationCenterDelegate {
    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
    ) -> Bool {
        UNUserNotificationCenter.current().delegate = self
        MainViewControllerKt.StartIosRuntime()
        return true
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        MainViewControllerKt.RecordNotificationDelivery(
            notificationId: notification.request.identifier
        )
        completionHandler([.banner, .sound])
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        defer { completionHandler() }
        MainViewControllerKt.RecordNotificationDelivery(
            notificationId: response.notification.request.identifier
        )
        guard let deepLink = response.notification.request.content.userInfo["deepLink"] as? String else { return }
        MainViewControllerKt.HandleDeepLink(url: deepLink)
    }
}

struct ComposeView: UIViewControllerRepresentable {
    func makeUIViewController(context: Context) -> UIViewController {
        MainViewControllerKt.MainViewController()
    }

    func updateUIViewController(_ uiViewController: UIViewController, context: Context) {}
}
