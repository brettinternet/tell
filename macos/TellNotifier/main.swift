// Tell.app posts one desktop notification attributed to Tell and exits. When
// the notification is clicked, macOS relaunches the app, which replays the
// click actions Tell stored in userInfo and exits. Tell computes every action;
// this helper only executes the stored argv lists.
//
// Usage:
//   Tell post    JSON payload on stdin: {"title", "body", "actions"}
//   Tell status  prints authorized, denied, not_determined, or provisional
//   Tell         launched by macOS for a click; otherwise requests permission
//
// Exit codes: 0 success, 3 notification permission denied, 1 other failure.
import AppKit
import UserNotifications

struct Action: Codable {
    let argv: [String]
    let env: [String]?
    let dir: String?
}

struct Payload: Decodable {
    let title: String
    let body: String
    let actions: [Action]?
}

final class Helper: NSObject, NSApplicationDelegate, UNUserNotificationCenterDelegate {
    let center = UNUserNotificationCenter.current()
    let mode = CommandLine.arguments.dropFirst().first ?? ""

    func applicationDidFinishLaunching(_ notification: Notification) {
        center.delegate = self
        switch mode {
        case "post": post()
        case "status": status()
        case "":
            // A click response arrives through the delegate shortly after a
            // relaunch. Without one, this was an ordinary launch: request
            // permission so the first notification is not lost to the prompt.
            DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
                self.center.requestAuthorization(options: [.alert, .sound]) { _, _ in exit(0) }
            }
        default: exit(1)
        }
    }

    func post() {
        guard let payload = try? JSONDecoder().decode(Payload.self, from: FileHandle.standardInput.readDataToEndOfFile()) else {
            exit(1)
        }
        center.requestAuthorization(options: [.alert, .sound]) { granted, _ in
            guard granted else { exit(3) }
            let content = UNMutableNotificationContent()
            content.title = payload.title
            content.body = payload.body
            if let actions = payload.actions, !actions.isEmpty,
               let encoded = try? JSONEncoder().encode(actions) {
                content.userInfo = ["actions": String(decoding: encoded, as: UTF8.self)]
            }
            let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
            self.center.add(request) { error in exit(error == nil ? 0 : 1) }
        }
    }

    func status() {
        center.getNotificationSettings { settings in
            let names: [UNAuthorizationStatus: String] = [
                .authorized: "authorized", .denied: "denied", .notDetermined: "not_determined",
                .provisional: "provisional",
            ]
            print(names[settings.authorizationStatus] ?? "unknown")
            exit(0)
        }
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse,
                                withCompletionHandler completionHandler: @escaping () -> Void) {
        if response.actionIdentifier == UNNotificationDefaultActionIdentifier,
           let text = response.notification.request.content.userInfo["actions"] as? String,
           let actions = try? JSONDecoder().decode([Action].self, from: Data(text.utf8)) {
            for action in actions { run(action) }
        }
        completionHandler()
        exit(0)
    }

    // Actions run in order; a failing action does not stop later ones.
    func run(_ action: Action) {
        guard let executable = action.argv.first else { return }
        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = Array(action.argv.dropFirst())
        var environment = ProcessInfo.processInfo.environment
        for entry in action.env ?? [] {
            let parts = entry.split(separator: "=", maxSplits: 1).map(String.init)
            if parts.count == 2 { environment[parts[0]] = parts[1] }
        }
        process.environment = environment
        if let dir = action.dir { process.currentDirectoryURL = URL(fileURLWithPath: dir) }
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        guard (try? process.run()) != nil else { return }
        // Bound each action so a hung command cannot keep the helper alive.
        let deadline = Date().addingTimeInterval(10)
        while process.isRunning && Date() < deadline { usleep(50_000) }
        if process.isRunning { process.terminate() }
    }
}

let app = NSApplication.shared
let helper = Helper()
app.delegate = helper
app.setActivationPolicy(.prohibited)
app.run()
