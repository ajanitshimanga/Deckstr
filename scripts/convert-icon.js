/**
 * Icon Conversion Script
 * Converts logo.svg to all required icon formats
 *
 * Usage: node scripts/convert-icon.js
 */

const sharp = require('sharp');
const pngToIco = require('png-to-ico').default;
const fs = require('fs');
const path = require('path');
const os = require('os');

const BUILD_DIR = path.join(__dirname, '..', 'build');
const WINDOWS_DIR = path.join(BUILD_DIR, 'windows');
const FRONTEND_ASSETS = path.join(__dirname, '..', 'frontend', 'src', 'assets', 'images');

const SVG_PATH = path.join(BUILD_DIR, 'logo.svg');

async function convertIcons() {
  console.log('Converting icons from logo.svg...\n');

  // Ensure directories exist
  if (!fs.existsSync(WINDOWS_DIR)) {
    fs.mkdirSync(WINDOWS_DIR, { recursive: true });
  }
  if (!fs.existsSync(FRONTEND_ASSETS)) {
    fs.mkdirSync(FRONTEND_ASSETS, { recursive: true });
  }

  // Read SVG
  const svgBuffer = fs.readFileSync(SVG_PATH);

  // 1. Generate appicon.png (256x256) for Wails
  console.log('1. Creating build/appicon.png (256x256)...');
  const appiconPath = path.join(BUILD_DIR, 'appicon.png');
  await sharp(svgBuffer)
    .resize(256, 256)
    .png()
    .toFile(appiconPath);
  console.log('   ✓ appicon.png created');

  // 2. Generate multiple PNG sizes for ICO (save to temp files)
  const sizes = [16, 32, 48, 256];
  const tempDir = path.join(os.tmpdir(), 'osm-icons');
  if (!fs.existsSync(tempDir)) {
    fs.mkdirSync(tempDir, { recursive: true });
  }

  const tempFiles = [];
  console.log('\n2. Creating PNG files for ICO conversion...');
  for (const size of sizes) {
    const tempPath = path.join(tempDir, `icon-${size}.png`);
    await sharp(svgBuffer)
      .resize(size, size)
      .png()
      .toFile(tempPath);
    tempFiles.push(tempPath);
    console.log(`   ✓ ${size}x${size} PNG created`);
  }

  // 3. Convert to ICO using file paths
  console.log('\n3. Creating build/windows/icon.ico...');
  const icoBuffer = await pngToIco(tempFiles);
  fs.writeFileSync(path.join(WINDOWS_DIR, 'icon.ico'), icoBuffer);
  console.log('   ✓ icon.ico created');

  // Cleanup temp files
  for (const tempFile of tempFiles) {
    fs.unlinkSync(tempFile);
  }

  // 4. Copy to frontend assets
  console.log('\n4. Creating frontend/src/assets/images/logo-universal.png...');
  await sharp(svgBuffer)
    .resize(512, 512)
    .png()
    .toFile(path.join(FRONTEND_ASSETS, 'logo-universal.png'));
  console.log('   ✓ logo-universal.png created');

  // 5. Also save the SVG to frontend for potential use
  console.log('\n5. Copying SVG to frontend assets...');
  fs.copyFileSync(SVG_PATH, path.join(FRONTEND_ASSETS, 'logo.svg'));
  console.log('   ✓ logo.svg copied');

  console.log('\n✅ All icons converted successfully!\n');
  console.log('Files updated:');
  console.log('  - build/appicon.png');
  console.log('  - build/windows/icon.ico');
  console.log('  - frontend/src/assets/images/logo-universal.png');
  console.log('  - frontend/src/assets/images/logo.svg');
}

convertIcons().catch(err => {
  console.error('Error converting icons:', err);
  process.exit(1);
});
