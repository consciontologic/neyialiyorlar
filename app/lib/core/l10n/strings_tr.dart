/// Turkish UI strings and metric metadata for the neyialiyorlar dashboard.
///
/// Metric descriptions are shown as hover tooltips on every metric card so
/// the user understands what each value means without leaving the screen.
library strings_tr;

// ─── Genel UI ────────────────────────────────────────────────────────────────

const kAppTitle = 'neyialiyorlar';

const kScreenDashboard = 'Piyasa';
const kScreenStocks = 'Hisseler';
const kScreenInvestigate = 'Sistem'; // kept for internal admin screen

const kSectionStocks = 'Hisseler';

const kLabelFund = 'Endeks';
const kLabelIsin = 'ISIN';
const kLabelEntityId = 'Varlık kimliği';
const kLabelType = 'Tür';
const kLabelStandalone = 'Endeks dışı';
const kLabelThisFund = 'Bu bir endeks / sepet varlığıdır';
const kLabelMetricsTracked = 'metrik izleniyor — canlı güncelleniyor';
const kLabelNoData = 'Veri yok';
const kLabelLoading = 'Yükleniyor…';

const kLabelSyntheticBanner =
    'Sentetik başlangıç verisi — değerler açıkça etiketlenmiş (yaklaşık) '
    'proxy\'lerdir, gerçek ölçümler değildir.';

const kLabelRefresh = 'Yenile';
const kLabelSearch = 'Hisse ara…';
const kLabelNoResults = 'Eşleşen hisse bulunamadı';

// Dashboard sections
const kSectionMarketSummary = 'Piyasa Özeti';
const kSectionTopNimvi = 'En Güçlü Hisseler';
const kSectionTopMomentum = 'En Hızlı Yükselen';
// Home page ranking: stocks ordered by how many funds hold them.
const kSectionMostHeld = 'Fonların En Çok Tuttuğu Hisseler';
const kLabelMostHeldEmpty = 'Henüz fon verisi yüklenmedi';
const kLabelTrackedStocks = 'Takip Edilen';
const kLabelXu030Count = 'BIST30\'da';
const kLabelDailyData = 'Günlük';

// Fund holdings section
const kSectionFundHoldings = 'Fon Portföyleri';
const kLabelNoFundData = 'Bu hisse için fon verisi henüz yüklenmedi';
const kLabelFundWeight = 'Ağırlık';
const kLabelFundSource = 'Kaynak';
// Has-funds / no-funds badge on the stock list
const kLabelNoFunds = 'Fon yok';
const kLabelFundsUnit = 'fon'; // e.g. "12 fon"

// Fund presence chart (real-time breadth: share of reporting funds holding it)
const kSectionFundPresence = 'Fon Yoğunluğu';
const kLabelPresenceSubtitle =
    'Bu hisseyi portföyünde tutan fonların, raporlayan tüm fonlara oranı';
const kLabelPresenceEmpty = 'Bu hisse için fon yoğunluğu verisi henüz oluşmadı';
const kLabelPresenceAsOf = 'Son veri';
const kLabelPresenceLive = 'Canlı';
const kLabelPresenceHolding = 'fon tutuyor'; // "12 / 400 fon tutuyor"
const kLabelChangeDaily = 'Günlük';
const kLabelChangeWeekly = 'Haftalık';
const kLabelChangeMonthly = 'Aylık';
const kLabelChangeYearly = 'Yıllık';

// Investigate screen
const kSectionSystemStatus = 'Sistem Durumu';
const kSectionMetricExplorer = 'Metrik Gezgini';
const kSectionTweak = 'Ayarla / Test Et';
const kLabelService = 'Servis';
const kLabelSyntheticEngine = 'Sentetik motor';
const kLabelEnabled = 'etkin';
const kLabelDisabled = 'devre dışı';
const kLabelMetricsCatalog = 'Katalogdaki metrik sayısı';
const kLabelRowCounts = 'Satır sayıları';
const kLabelMetric = 'Metrik';
const kLabelEntity = 'Varlık';
const kLabelWindow = 'Pencere';
const kLabelGenerateTick = 'Veri Üret';
const kLabelTickEnabled =
    'Her metrik/varlık çifti için bir sentetik gözlem üretir '
    've canlı WebSocket üzerinden iletir.';
const kLabelTickDisabled =
    'Sentetik motor devre dışı; yeniden hesaplama kullanılamaz.';

const kLabelRawLatest = 'Ham son değer (JSON)';
const kLabelNoLatest = '// son değer yok';
const kLabelSeriesPoints = 'nokta';
const kLabelNoSeriesPoints = 'Bu seçim için seri noktası bulunamadı.';
const kLabelStatusUnavailable = 'Durum alınamadı';
const kLabelMetricsUnavailable = 'Metrikler alınamadı';

// Confidence flags
const kFlagFresh = 'Güncel';
const kFlagStale = 'Eski';
const kFlagApprox = 'Yaklaşık';

String confidenceTR(String flag) => switch (flag) {
      'fresh' => kFlagFresh,
      'stale' => kFlagStale,
      'approx' => kFlagApprox,
      _ => flag,
    };

// ─── Metrik Türkçe İsimleri ───────────────────────────────────────────────────

/// Returns the Turkish display name for a metric key.
/// Falls back to the English display name passed as [fallback].
String metricNameTR(String key, {String fallback = ''}) =>
    _kMetricNamesTR[key] ?? (fallback.isNotEmpty ? fallback : key);

const Map<String, String> _kMetricNamesTR = {
  'velocity_accumulation': 'Alım Hızı',
  'herding_index': 'Sürü Etkisi',
  'basis_spread': 'Piyasa Baskısı',
  'float_demand_ratio': 'Talep Yüksekliği',
  'property_equity_ratio': 'Varlık/Özsermaye',
  'foreign_accum_velocity': 'Yabancı Alımı',
  'mandate_expansion': 'Kurum Talebi',
  'nimvi': 'Güç Skoru',
  'kap_notification': 'Haberler',
  'real_yield_divergence': 'Reel Getiri',
  'price_close': 'Kapanış Fiyatı',
};

// ─── Metrik Açıklamaları (tooltip) ───────────────────────────────────────────

/// Returns the Turkish description for a metric key; used as hover tooltip.
String? metricDescTR(String key) => _kMetricDescTR[key];

const Map<String, String> _kMetricDescTR = {
  'velocity_accumulation': 'Son günlerde bu hisse ne kadar yoğun alındı? '
      'Yüksekse alımlar hızlanıyor demektir.',
  'herding_index': 'Büyük yatırımcılar aynı anda aynı yöne mi gidiyor? '
      'Yüksek değer sürü hareketi anlamına gelir.',
  'basis_spread': 'Spot fiyat ile vadeli fiyat arasındaki uçurum. '
      'Genişse piyasada gerginlik var demektir.',
  'float_demand_ratio':
      'Bu hisse için talep, piyasadaki mevcut arza göre ne kadar yüksek? '
          '1\'in üstü kıtlık sinyali.',
  'property_equity_ratio':
      'Şirketin binadaki / arazideki varlığının özsermayeye oranı. '
          'Yüksekse varlıklı ama kıt nakitli olabilir.',
  'foreign_accum_velocity':
      'Yabancı yatırımcılar bu hisseyi alıyor mu satıyor mu, ne hızla? '
          'Pozitif → alım, negatif → satış.',
  'mandate_expansion': 'Kurumsal fonlar portföyüne bu hisseyi ekliyor mu? '
      'Yüksekse kurumsal ilgi artıyor.',
  'nimvi': 'Hacim ve nakit akışını birleştiren tek bir güç skoru. '
      'Yüksek → hisse güçlü, düşük → zayıf.',
  'kap_notification': 'Bugün bu şirketle ilgili kaç önemli açıklama yapıldı? '
      'Yüksek değer = önemli bir şey oluyor.',
  'real_yield_divergence':
      'Bono getirisi ile gerçek enflasyon arasındaki fark. '
          'Pozitifse yatırımcı ihtiyatlı duruyor.',
  'price_close': 'En son kapanış fiyatı (TL). Her gün alınır.',
};
