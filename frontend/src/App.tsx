import AboutDialog from "./components/AboutDialog";
import NetworkPrinting from "./components/NetworkPrinting";
import NetworkPrintingEnabledDialog from "./components/NetworkPrintingEnabledDialog";
import PrinterList from "./components/PrinterList";
import UpdateBanner from "./components/UpdateBanner";
import { AppContextWrapper } from "./contexts/AppContext";
import { PrinterContextWrapper } from "./contexts/PrinterContext";
import { ToastContextWrapper } from "./contexts/ToastContext";

function App() {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <PrinterContextWrapper>
          <div className="min-h-screen flex flex-col items-center justify-between p-4 sm:p-6 font-sans bg-gray-50">
            <div className="w-full flex flex-col items-center justify-center flex-1">
              <PrinterList />
              <NetworkPrintingEnabledDialog />
              <NetworkPrinting />
              <UpdateBanner />
            </div>
            <div className="mt-4 pb-2">
              <AboutDialog />
            </div>
          </div>
        </PrinterContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}

export default App;
