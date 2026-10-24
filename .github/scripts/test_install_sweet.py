import hashlib
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
import zipfile


spec = importlib.util.spec_from_file_location("installer", Path(__file__).with_name("install-sweet.py"))
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class InstallerTests(unittest.TestCase):
    def test_platform_assets(self):
        for system, machine, suffix in [
            ("Linux", "x86_64", "linux_amd64.tar.gz"),
            ("Linux", "aarch64", "linux_arm64.tar.gz"),
            ("Darwin", "arm64", "darwin_arm64.tar.gz"),
            ("Windows", "AMD64", "windows_amd64.zip"),
            ("Windows", "ARM64", "windows_arm64.zip"),
        ]:
            self.assertEqual(installer.asset_name("1.2.3", system, machine), f"sweet_1.2.3_{suffix}")
        with self.assertRaises(KeyError):
            installer.asset_name("1.2.3", "Linux", "unknown")

    def test_extract_only_verified_executable(self):
        for extension, binary in [("tar.gz", "sweet"), ("zip", "sweet.exe")]:
            with self.subTest(extension=extension), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                archive = root / f"release.{extension}"
                payload = b"example executable"
                if extension == "zip":
                    with zipfile.ZipFile(archive, "w") as bundle:
                        bundle.writestr(binary, payload)
                        bundle.writestr("../escape", "unwanted")
                else:
                    with tarfile.open(archive, "w:gz") as bundle:
                        member = tarfile.TarInfo(binary)
                        member.size = len(payload)
                        bundle.addfile(member, io.BytesIO(payload))
                        member = tarfile.TarInfo("../escape")
                        member.size = 1
                        bundle.addfile(member, io.BytesIO(b"x"))
                checksums = root / "checksums.txt"
                checksum = f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n"
                checksums.write_text(checksum)
                target = installer.extract_binary(archive, checksums, root / "bin")
                self.assertEqual(target.read_bytes(), payload)
                self.assertFalse((root / "escape").exists())
                for invalid in ["", checksum * 2, "0" * 64 + f"  {archive.name}\n"]:
                    checksums.write_text(invalid)
                    with self.assertRaises(ValueError):
                        installer.extract_binary(archive, checksums, root / "rejected")
                    self.assertFalse((root / "rejected").exists())


if __name__ == "__main__":
    unittest.main()
