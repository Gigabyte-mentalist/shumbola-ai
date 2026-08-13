import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import uzLatn from './locales/uz-Latn.json'
import uzCyrl from './locales/uz-Cyrl.json'
import ru from './locales/ru.json'

// TZ 6.5: interfeys uch tilda — o'zbek (lotin), o'zbek (kirill), rus.
void i18n.use(initReactI18next).init({
  resources: {
    'uz-Latn': { translation: uzLatn },
    'uz-Cyrl': { translation: uzCyrl },
    ru: { translation: ru },
  },
  lng: 'uz-Latn',
  fallbackLng: 'uz-Latn',
  interpolation: { escapeValue: false },
})

export default i18n
