// Renders the 1024px Tell.app icon PNG to the path in argv[1] at build time,
// so the repository carries no binary artwork.
import AppKit

let size = 1024.0
let image = NSImage(size: NSSize(width: size, height: size))
image.lockFocus()
let inset = size * 0.1
let tile = NSRect(x: inset, y: inset, width: size - 2 * inset, height: size - 2 * inset)
NSColor(calibratedRed: 0.16, green: 0.42, blue: 0.95, alpha: 1).setFill()
NSBezierPath(roundedRect: tile, xRadius: tile.width * 0.225, yRadius: tile.width * 0.225).fill()
let configuration = NSImage.SymbolConfiguration(pointSize: size * 0.42, weight: .semibold)
    .applying(.init(paletteColors: [.white]))
if let symbol = NSImage(systemSymbolName: "bell.fill", accessibilityDescription: nil)?
    .withSymbolConfiguration(configuration) {
    let origin = NSPoint(x: (size - symbol.size.width) / 2, y: (size - symbol.size.height) / 2)
    symbol.draw(at: origin, from: .zero, operation: .sourceOver, fraction: 1)
}
image.unlockFocus()
guard let tiff = image.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else { exit(1) }
try png.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
