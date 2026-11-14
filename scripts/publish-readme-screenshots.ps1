<#
.SYNOPSIS
  Build README screenshot assets from actual captured Windows EXE pixels.
.DESCRIPTION
  No browser, Node.js or generated mockup. This script only uses PNG frames
  recorded by the visual-matrix GitHub Actions workflow.
#>
param(
    [string]$CaptureDir = 'visual-matrix',
    [string]$OutputDir = 'docs/ui'
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$captures = @(
    @{ From = 'settings-light-top.png'; To = 'settings-light.png' },
    @{ From = 'settings-dark-top.png'; To = 'settings-dark.png' },
    @{ From = 'answer-light-top.png'; To = 'result-card.png' },
    @{ From = 'recording-200-top.png'; To = 'recording-overlay.png' }
)

New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
foreach ($item in $captures) {
    $source = Join-Path $CaptureDir $item.From
    if (!(Test-Path -LiteralPath $source -PathType Leaf)) {
        throw "Missing real EXE screenshot: $source"
    }
    # Preserve the full unaltered HWND captures in the Actions artifact.
    # Documentation copies are trimmed only to remove pixels belonging
    # to the desktop behind the DWM/window border (e.g. console text).
    # The actual controls and every interior pixel come from the real EXE.
    $image = [System.Drawing.Image]::FromFile((Resolve-Path $source).Path)
    try {
        if ($image.Width -lt 100 -or $image.Height -lt 30) {
            throw "Unexpected screenshot dimensions: $source ($($image.Width)x$($image.Height))"
        }
        $left = 8
        $right = 8
        $top = 0
        $bottom = 10
        if ($item.To -eq 'recording-overlay.png') {
            $left = 11
            $right = 11
            $top = 10
            $bottom = 10
        }
        $cropWidth = $image.Width - $left - $right
        $cropHeight = $image.Height - $top - $bottom
        if ($cropWidth -lt 100 -or $cropHeight -lt 30) {
            throw "Screenshot too small after border trimming: $source"
        }
        $clean = [System.Drawing.Bitmap]::new($cropWidth, $cropHeight)
        $painter = [System.Drawing.Graphics]::FromImage($clean)
        try {
            $dest = [System.Drawing.Rectangle]::new(0, 0, $cropWidth, $cropHeight)
            $sourceRect = [System.Drawing.Rectangle]::new($left, $top, $cropWidth, $cropHeight)
            $painter.DrawImage($image, $dest, $sourceRect, [System.Drawing.GraphicsUnit]::Pixel)
            $clean.Save((Join-Path (Get-Location) (Join-Path $OutputDir $item.To)), [System.Drawing.Imaging.ImageFormat]::Png)
        } finally {
            $painter.Dispose()
            $clean.Dispose()
        }
    } finally {
        $image.Dispose()
    }
}

function UiColor([string]$hex) {
    return [System.Drawing.ColorTranslator]::FromHtml($hex)
}
function DrawText([string]$text, [float]$x, [float]$y, [System.Drawing.Font]$font, [string]$color) {
    $brush = [System.Drawing.SolidBrush]::new((UiColor $color))
    try {
        $graphics.DrawString($text, $font, $brush, $x, $y)
    } finally {
        $brush.Dispose()
    }
}
function DrawPanel([int]$x, [int]$y, [int]$width, [int]$height) {
    $background = [System.Drawing.SolidBrush]::new((UiColor '#1B273A'))
    $edge = [System.Drawing.Pen]::new((UiColor '#344259'), 2)
    try {
        $rect = [System.Drawing.Rectangle]::new($x, $y, $width, $height)
        $graphics.FillRectangle($background, $rect)
        $graphics.DrawRectangle($edge, $rect)
    } finally {
        $background.Dispose()
        $edge.Dispose()
    }
}
function DrawScreenshot([string]$file, [int]$x, [int]$y, [int]$width, [int]$height) {
    $image = [System.Drawing.Image]::FromFile((Resolve-Path $file).Path)
    try {
        $ratio = [Math]::Min([double]$width / $image.Width, [double]$height / $image.Height)
        $drawWidth = [int][Math]::Round($image.Width * $ratio)
        $drawHeight = [int][Math]::Round($image.Height * $ratio)
        $drawX = $x + [int][Math]::Floor(($width - $drawWidth) / 2)
        $drawY = $y + [int][Math]::Floor(($height - $drawHeight) / 2)
        $rect = [System.Drawing.Rectangle]::new($drawX, $drawY, $drawWidth, $drawHeight)
        $graphics.DrawImage($image, $rect)
    } finally {
        $image.Dispose()
    }
}

$canvas = [System.Drawing.Bitmap]::new(1600, 1100)
$graphics = [System.Drawing.Graphics]::FromImage($canvas)
$titleFont = [System.Drawing.Font]::new('Segoe UI', 56, [System.Drawing.FontStyle]::Bold, [System.Drawing.GraphicsUnit]::Pixel)
$labelFont = [System.Drawing.Font]::new('Segoe UI', 27, [System.Drawing.FontStyle]::Bold, [System.Drawing.GraphicsUnit]::Pixel)
$noteFont = [System.Drawing.Font]::new('Segoe UI', 21, [System.Drawing.FontStyle]::Regular, [System.Drawing.GraphicsUnit]::Pixel)
try {
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $graphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAlias
    $graphics.Clear((UiColor '#0F1728'))

    DrawText 'VoiceRewrite' 52 38 $titleFont '#F6F8FF'
    DrawText 'VOICE TYPING   /   AI REWRITING   /   NATIVE WINDOWS 11' 59 113 $noteFont '#C4CCDE'

    DrawPanel 48 165 720 875
    DrawText '01  CONNECT & CONFIGURE' 79 187 $labelFont '#F2F6FF'
    DrawScreenshot (Join-Path $OutputDir 'settings-light.png') 83 250 645 660
    DrawText 'ASR  +  LLM  +  microphone  +  hotkeys' 88 940 $noteFont '#B9C5DA'

    DrawPanel 792 165 760 555
    DrawText '02  REVIEW AI RESULTS' 822 189 $labelFont '#F2F6FF'
    DrawScreenshot (Join-Path $OutputDir 'result-card.png') 823 246 698 404
    DrawText 'Rewrite, translate or ask; copy the full result.' 838 657 $noteFont '#B9C5DA'

    DrawPanel 792 747 760 293
    DrawText '03  HOLD TO SPEAK' 822 770 $labelFont '#F2F6FF'
    DrawScreenshot (Join-Path $OutputDir 'recording-overlay.png') 827 833 690 143
    DrawText 'Minimal, always-on-top recording pill.' 838 986 $noteFont '#B9C5DA'

    $target = Join-Path $OutputDir 'overview.png'
    $canvas.Save((Join-Path (Get-Location) $target), [System.Drawing.Imaging.ImageFormat]::Png)
    Write-Host "Published five real-EXE-derived image files to $OutputDir"
} finally {
    $titleFont.Dispose()
    $labelFont.Dispose()
    $noteFont.Dispose()
    $graphics.Dispose()
    $canvas.Dispose()
}
