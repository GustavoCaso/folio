from pathlib import Path
from unittest.mock import MagicMock, call, patch

from docling.datamodel.pipeline_options import TableFormerMode
from docling_core.types.doc.document import DoclingDocument

import parser.formats.converter as converter_mod
from parser.formats.converter import (
    convert,
    convert_pdf_batched,
    convert_pdf_page_range,
    html_backend_options,
    pdf_page_batches,
    pdf_pipeline_options,
)


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

    def test_table_structure_mode_default_accurate(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.table_structure_options.mode == TableFormerMode.ACCURATE

    def test_table_structure_mode_fast(self):
        with patch.dict("os.environ", {"PDF_TABLE_STRUCTURE_MODE": "fast"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.table_structure_options.mode == TableFormerMode.FAST

    def test_table_structure_mode_unknown_falls_back_to_accurate(self):
        with patch.dict("os.environ", {"PDF_TABLE_STRUCTURE_MODE": "bogus"}, clear=True):
            opts = pdf_pipeline_options()
            assert opts.table_structure_options.mode == TableFormerMode.ACCURATE


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


class TestPdfPageBatches:
    def test_single_batch_when_pages_fit(self):
        with patch.dict("os.environ", {"PDF_BATCH_SIZE": "100"}, clear=True):
            assert pdf_page_batches(50) == [(1, 50)]

    def test_exact_multiple(self):
        with patch.dict("os.environ", {"PDF_BATCH_SIZE": "100"}, clear=True):
            assert pdf_page_batches(200) == [(1, 100), (101, 200)]

    def test_splits_into_correct_ranges(self):
        with patch.dict("os.environ", {"PDF_BATCH_SIZE": "100"}, clear=True):
            assert pdf_page_batches(250) == [(1, 100), (101, 200), (201, 250)]

    def test_last_batch_clamped_to_page_count(self):
        with patch.dict("os.environ", {"PDF_BATCH_SIZE": "100"}, clear=True):
            batches = pdf_page_batches(110)
            assert batches[-1] == (101, 110)


class TestConvertPdfPageRange:
    def test_calls_converter_with_page_range(self):
        path = Path("/tmp/doc.pdf")
        mock_result = MagicMock()

        with patch.object(
            converter_mod._pdf_converter, "convert", return_value=mock_result
        ) as mock_convert:
            result = convert_pdf_page_range(path, 1, 50)

        mock_convert.assert_called_once_with(source=path, page_range=(1, 50))
        assert result is mock_result.document


class TestConvertPdfBatched:
    def _make_mock_doc(self, name: str) -> MagicMock:
        doc = MagicMock(spec=DoclingDocument)
        doc.name = name
        return doc

    def test_concatenates_all_batch_docs(self):
        path = Path("/tmp/doc.pdf")
        d1, d2 = self._make_mock_doc("d1"), self._make_mock_doc("d2")
        merged = MagicMock()

        with (
            patch.object(converter_mod, "convert_pdf_page_range", side_effect=[d1, d2]),
            patch.object(
                converter_mod.DoclingDocument, "concatenate", return_value=merged
            ) as mock_concat,
            patch.dict("os.environ", {"PDF_BATCH_SIZE": "100"}, clear=True),
        ):
            result = convert_pdf_batched(path, page_count=150)

        mock_concat.assert_called_once_with([d1, d2])
        assert result is merged

    def test_uses_pdf_page_batches_for_ranges(self):
        path = Path("/tmp/doc.pdf")
        d1, d2, d3 = (self._make_mock_doc(f"d{i}") for i in range(3))

        with (
            patch.object(
                converter_mod, "convert_pdf_page_range", side_effect=[d1, d2, d3]
            ) as mock_range,
            patch.object(converter_mod.DoclingDocument, "concatenate", return_value=MagicMock()),
            patch.dict("os.environ", {"PDF_BATCH_SIZE": "100"}, clear=True),
        ):
            convert_pdf_batched(path, page_count=250)

        mock_range.assert_has_calls(
            [
                call(path, 1, 100),
                call(path, 101, 200),
                call(path, 201, 250),
            ]
        )


class TestConvert:
    def test_returns_docling_document_for_url(self):
        mock_document = MagicMock(spec=DoclingDocument)
        mock_result = MagicMock()
        mock_result.document = mock_document

        mock_html_converter = MagicMock()
        mock_html_converter.convert.return_value = mock_result

        with patch.object(
            converter_mod, "_make_html_converter", return_value=mock_html_converter
        ) as mock_make:
            result = convert("https://example.com/page")

        assert result is mock_document
        mock_make.assert_called_once_with("https://example.com")

    def test_calls_pdf_converter_for_path(self):
        mock_result = MagicMock()
        path = Path("/tmp/doc.pdf")

        with patch.object(
            converter_mod._pdf_converter, "convert", return_value=mock_result
        ) as mock_convert:
            convert(path)

        mock_convert.assert_called_once_with(source=path)

    def test_calls_pdf_converter_for_pdf_url(self):
        mock_result = MagicMock()
        mock_result.document = MagicMock()

        with (
            patch.object(
                converter_mod._pdf_converter, "convert", return_value=mock_result
            ) as mock_convert,
            patch.object(converter_mod, "_make_html_converter") as mock_make,
        ):
            convert("https://example.com/paper.pdf")

        mock_convert.assert_called_once_with(source="https://example.com/paper.pdf")
        mock_make.assert_not_called()

    def test_make_html_converter_creates_fresh_instance_each_call(self):
        with patch("parser.formats.converter.DocumentConverter") as mock_cls:
            mock_cls.return_value = MagicMock()
            converter_mod._make_html_converter("https://example.com")
            converter_mod._make_html_converter("https://example.com")
        assert mock_cls.call_count == 2

    def test_source_uri_set_to_origin(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_backend_options("https://example.com")
        # Pydantic normalises URLs by appending a trailing slash
        assert str(opts.source_uri).rstrip("/") == "https://example.com"

    def test_source_uri_none_when_empty(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_backend_options("")
        assert opts.source_uri is None
