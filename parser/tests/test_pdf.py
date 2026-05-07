from unittest.mock import MagicMock, patch

import pytest
from docling_core.types.doc.base import ImageRefMode

import parser.formats.pdf as pdf_mod


class TestBoolEnv:
    def test_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            assert pdf_mod._bool_env("MISSING_VAR", False) is False

    def test_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            assert pdf_mod._bool_env("MISSING_VAR", True) is True

    @pytest.mark.parametrize("val", ["1", "true", "True", "TRUE", "yes", "Yes", "YES"])
    def test_truthy_values(self, val):
        with patch.dict("os.environ", {"SOME_VAR": val}, clear=True):
            assert pdf_mod._bool_env("SOME_VAR", False) is True

    @pytest.mark.parametrize("val", ["0", "false", "no", "off", ""])
    def test_falsy_values(self, val):
        with patch.dict("os.environ", {"SOME_VAR": val}, clear=True):
            assert pdf_mod._bool_env("SOME_VAR", True) is False


class TestImageMode:
    def test_default_is_placeholder(self):
        with patch.dict("os.environ", {}, clear=True):
            assert pdf_mod._image_mode() == ImageRefMode.PLACEHOLDER

    def test_embedded(self):
        with patch.dict("os.environ", {"PDF_IMAGE_MODE": "embedded"}, clear=True):
            assert pdf_mod._image_mode() == ImageRefMode.EMBEDDED

    def test_referenced(self):
        with patch.dict("os.environ", {"PDF_IMAGE_MODE": "referenced"}, clear=True):
            assert pdf_mod._image_mode() == ImageRefMode.REFERENCED

    def test_unknown_falls_back_to_placeholder(self):
        with patch.dict("os.environ", {"PDF_IMAGE_MODE": "bogus"}, clear=True):
            assert pdf_mod._image_mode() == ImageRefMode.PLACEHOLDER

    def test_case_insensitive(self):
        with patch.dict("os.environ", {"PDF_IMAGE_MODE": "EMBEDDED"}, clear=True):
            assert pdf_mod._image_mode() == ImageRefMode.EMBEDDED


class TestPipelineOptions:
    def test_generate_images_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.generate_picture_images is True

    def test_generate_images_true(self):
        with patch.dict("os.environ", {"PDF_GENERATE_IMAGES": "true"}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.generate_picture_images is True

    def test_do_ocr_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.do_ocr is True

    def test_do_ocr_disabled(self):
        with patch.dict("os.environ", {"PDF_DO_OCR": "false"}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.do_ocr is False

    def test_do_table_structure_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.do_table_structure is True

    def test_do_table_structure_disabled(self):
        with patch.dict("os.environ", {"PDF_DO_TABLE_STRUCTURE": "false"}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.do_table_structure is False

    def test_do_code_enrichment_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.do_code_enrichment is False

    def test_do_code_enrichment_enabled(self):
        with patch.dict("os.environ", {"PDF_DO_CODE_ENRICHMENT": "true"}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.do_code_enrichment is True

    def test_code_formula_preset_default_is_codeformulav2(self):
        with patch.dict("os.environ", {"PDF_DO_CODE_ENRICHMENT": "true"}, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.code_formula_options.model_spec.name == "CodeFormulaV2"

    def test_code_formula_preset_granite_docling(self):
        with patch.dict(
            "os.environ",
            {"PDF_DO_CODE_ENRICHMENT": "true", "PDF_CODE_FORMULA_PRESET": "granite_docling"},
            clear=True,
        ):
            opts = pdf_mod._pipeline_options()
            assert opts.code_formula_options.model_spec.name == "Granite-Docling-258M"

    def test_code_formula_preset_unknown_falls_back_to_default(self):
        with patch.dict(
            "os.environ",
            {"PDF_DO_CODE_ENRICHMENT": "true", "PDF_CODE_FORMULA_PRESET": "bogus"},
            clear=True,
        ):
            opts = pdf_mod._pipeline_options()
            assert opts.code_formula_options.model_spec.name == "CodeFormulaV2"

    def test_batch_size_vars(self):
        env = {
            "PDF_LAYOUT_BATCH_SIZE": "4",
            "PDF_OCR_BATCH_SIZE": "2",
            "PDF_TABLE_BATCH_SIZE": "3",
            "PDF_QUEUE_MAX_SIZE": "8",
        }
        with patch.dict("os.environ", env, clear=True):
            opts = pdf_mod._pipeline_options()
            assert opts.layout_batch_size == 4
            assert opts.ocr_batch_size == 2
            assert opts.table_batch_size == 3
            assert opts.queue_max_size == 8


class TestConvertPdf:
    def test_uses_configured_image_mode(self, tmp_path):
        pdf = tmp_path / "doc.pdf"
        pdf.write_bytes(b"%PDF")

        mock_result = MagicMock()
        mock_result.document.export_to_markdown.return_value = "# Hello"

        with (
            patch.dict("os.environ", {"PDF_IMAGE_MODE": "embedded"}, clear=True),
            patch.object(pdf_mod._converter, "convert", return_value=mock_result),
            patch.object(pdf_mod, "_image_mode_value", ImageRefMode.EMBEDDED),
        ):
            result = pdf_mod.convert_pdf(pdf)

        mock_result.document.export_to_markdown.assert_called_once_with(
            image_mode=ImageRefMode.EMBEDDED
        )
        assert result == "# Hello"

    def test_enrich_code_blocks_when_enabled(self, tmp_path):
        pdf = tmp_path / "doc.pdf"
        pdf.write_bytes(b"%PDF")

        mock_result = MagicMock()
        mock_result.document.export_to_markdown.return_value = "```\nprint('hi')\n```"

        with (
            patch.object(pdf_mod._converter, "convert", return_value=mock_result),
            patch.object(pdf_mod, "_image_mode_value", ImageRefMode.PLACEHOLDER),
            patch.object(pdf_mod, "_post_process_code_blocks", True),
        ):
            result = pdf_mod.convert_pdf(pdf)

        assert result.startswith("```py")

    def test_enrich_code_blocks_skipped_when_disabled(self, tmp_path):
        pdf = tmp_path / "doc.pdf"
        pdf.write_bytes(b"%PDF")

        mock_result = MagicMock()
        mock_result.document.export_to_markdown.return_value = "```\nprint('hi')\n```"

        with (
            patch.object(pdf_mod._converter, "convert", return_value=mock_result),
            patch.object(pdf_mod, "_image_mode_value", ImageRefMode.PLACEHOLDER),
            patch.object(pdf_mod, "_post_process_code_blocks", False),
        ):
            result = pdf_mod.convert_pdf(pdf)

        assert result == "```\nprint('hi')\n```"
