from pathlib import Path
from unittest.mock import MagicMock, patch

from parser.formats.converter import _converter, convert, html_backend_options, pdf_pipeline_options


class TestPDFPipelineOptions:
    def test_do_ocr_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.do_ocr is True

    def test_do_ocr_disabled(self):
        with patch.dict("os.environ", {"PDF_DO_OCR": "false"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.do_ocr is False

    def test_do_table_structure_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.do_table_structure is True

    def test_do_table_structure_disabled(self):
        with patch.dict("os.environ", {"PDF_DO_TABLE_STRUCTURE": "false"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.do_table_structure is False

    def test_do_code_enrichment_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.do_code_enrichment is False

    def test_do_code_enrichment_enabled(self):
        with patch.dict("os.environ", {"PDF_DO_CODE_ENRICHMENT": "true"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.do_code_enrichment is True

    def test_code_formula_preset_default_is_codeformulav2(self):
        with patch.dict("os.environ", {"PDF_DO_CODE_ENRICHMENT": "true"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.code_formula_options.model_spec.name == "CodeFormulaV2"

    def test_code_formula_preset_granite_docling(self):
        with patch.dict(
            "os.environ",
            {"PDF_DO_CODE_ENRICHMENT": "true", "PDF_CODE_FORMULA_PRESET": "granite_docling"},
            clear=True,
        ):
            opts = pdf_pipeline_options()
            assert opts.code_formula_options.model_spec.name == "Granite-Docling-258M"

    def test_code_formula_preset_unknown_falls_back_to_default(self):
        with patch.dict(
            "os.environ",
            {"PDF_DO_CODE_ENRICHMENT": "true", "PDF_CODE_FORMULA_PRESET": "bogus"},
            clear=True,
        ):
            opts = pdf_pipeline_options()
            assert opts.code_formula_options.model_spec.name == "CodeFormulaV2"

    def test_images_scale_default_is_1(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.images_scale == 1.0

    def test_images_scale_set_from_env(self):
        with patch.dict("os.environ", {"PDF_IMAGES_SCALE": "0.5"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.images_scale == 0.5

    def test_generate_page_images_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.generate_page_images is False

    def test_generate_page_images_enabled(self):
        with patch.dict("os.environ", {"PDF_GENERATE_PAGE_IMAGES": "true"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.generate_page_images is True

    def test_do_formula_enrichment_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.do_formula_enrichment is False

    def test_do_formula_enrichment_enabled(self):
        with patch.dict("os.environ", {"PDF_DO_FORMULA_ENRICHMENT": "true"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.do_formula_enrichment is True

    def test_force_backend_text_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.force_backend_text is False

    def test_force_backend_text_enabled(self):
        with patch.dict("os.environ", {"PDF_FORCE_BACKEND_TEXT": "true"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.force_backend_text is True

    def test_document_timeout_default_none(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.document_timeout is None

    def test_document_timeout_set_from_env(self):
        with patch.dict("os.environ", {"PDF_DOCUMENT_TIMEOUT": "120"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.document_timeout == 120.0

    def test_num_threads_default_is_docling_default(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.accelerator_options.num_threads == 4

    def test_num_threads_set_from_env(self):
        with patch.dict("os.environ", {"PDF_NUM_THREADS": "2"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.accelerator_options.num_threads == 2

    def test_batch_size_vars(self):
        env = {
            "PDF_LAYOUT_BATCH_SIZE": "4",
            "PDF_OCR_BATCH_SIZE": "2",
            "PDF_TABLE_BATCH_SIZE": "3",
            "PDF_QUEUE_MAX_SIZE": "8",
        }
        with patch.dict("os.environ", env, clear=True):
            opts = pdf_pipeline_options()
            assert opts.layout_batch_size == 4
            assert opts.ocr_batch_size == 2
            assert opts.table_batch_size == 3
            assert opts.queue_max_size == 8


class TestHTMLPipelineOptions:
    def test_fetch_images_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_backend_options()
            assert opts.fetch_images is False

    def test_fetch_images_enabled(self):
        with patch.dict("os.environ", {"HTML_FETCH_IMAGES": "true"}, clear=True):
            opts = html_backend_options()
            assert opts.fetch_images is True

    def test_render_page_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_backend_options()
            assert opts.render_page is False

    def test_render_page_enabled(self):
        with patch.dict("os.environ", {"HTML_RENDER_PAGE": "true"}, clear=True):
            opts = html_backend_options()
            assert opts.render_page is True

    def test_render_dpi_default_96(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_backend_options()
            assert opts.render_dpi == 96

    def test_render_dpi_set_from_env(self):
        with patch.dict("os.environ", {"HTML_RENDER_DPI": "72"}, clear=True):
            opts = html_backend_options()
            assert opts.render_dpi == 72

    def test_render_device_scale_default_1(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_backend_options()
            assert opts.render_device_scale == 1.0

    def test_render_device_scale_set_from_env(self):
        with patch.dict("os.environ", {"HTML_RENDER_DEVICE_SCALE": "0.5"}, clear=True):
            opts = html_backend_options()
            assert opts.render_device_scale == 0.5

    def test_add_title_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_backend_options()
            assert opts.add_title is True

    def test_add_title_disabled(self):
        with patch.dict("os.environ", {"HTML_ADD_TITLE": "false"}, clear=True):
            opts = html_backend_options()
            assert opts.add_title is False

    def test_infer_furniture_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_backend_options()
            assert opts.infer_furniture is True

    def test_infer_furniture_disabled(self):
        with patch.dict("os.environ", {"HTML_INFER_FURNITURE": "false"}, clear=True):
            opts = html_backend_options()
            assert opts.infer_furniture is False


class TestConvert:
    def test_returns_docling_document(self):
        from unittest.mock import MagicMock

        from docling_core.types.doc.document import DoclingDocument

        mock_document = MagicMock(spec=DoclingDocument)
        mock_result = MagicMock()
        mock_result.document = mock_document

        with patch.object(_converter, "convert", return_value=mock_result):
            result = convert("https://example.com/doc.pdf")

        assert result is mock_document

    def test_calls_converter_with_source(self):
        mock_result = MagicMock()
        path = Path("/tmp/doc.pdf")

        with patch.object(_converter, "convert", return_value=mock_result) as mock_convert:
            convert(path)

        mock_convert.assert_called_once_with(source=path)
